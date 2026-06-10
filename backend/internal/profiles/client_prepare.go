package profiles

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"launcher-backend/internal/models"
)

type PrepareClientResult struct {
	Profile    ProfileSummary `json:"profile"`
	FileCount  int64          `json:"fileCount"`
	TotalSize  int64          `json:"totalSize"`
	Downloaded int            `json:"downloaded"`
	Message    string         `json:"message"`
}

type versionManifest struct {
	Versions []struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"versions"`
}

type minecraftVersion struct {
	ID        string `json:"id"`
	Downloads struct {
		Client downloadInfo `json:"client"`
	} `json:"downloads"`
	AssetIndex downloadInfo `json:"assetIndex"`
	Libraries  []struct {
		Name      string `json:"name"`
		Downloads struct {
			Artifact    *artifactInfo           `json:"artifact"`
			Classifiers map[string]artifactInfo `json:"classifiers"`
		} `json:"downloads"`
	} `json:"libraries"`
	Logging struct {
		Client struct {
			File artifactInfo `json:"file"`
		} `json:"client"`
	} `json:"logging"`
}

type downloadInfo struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

type artifactInfo struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	ID   string `json:"id"`
}

type assetIndex struct {
	Objects map[string]struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	} `json:"objects"`
}

type loaderProfile struct {
	ID        string `json:"id"`
	Libraries []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
		SHA1 string `json:"sha1"`
		Size int64  `json:"size"`
	} `json:"libraries"`
}

type installerArtifact struct {
	fileName string
	url      string
}

func (s Service) PrepareClient(ctx context.Context, id string) (PrepareClientResult, error) {
	var profile models.Profile
	if err := s.db.WithContext(ctx).First(&profile, "id = ?", id).Error; err != nil {
		return PrepareClientResult{}, err
	}
	if err := validateSlug(profile.Slug); err != nil {
		return PrepareClientResult{}, err
	}

	root := s.filesRoot(profile)
	if err := os.MkdirAll(root, 0755); err != nil {
		return PrepareClientResult{}, err
	}

	downloader := profileDownloader{
		client: http.Client{Timeout: 60 * time.Second},
		root:   root,
	}

	if err := downloader.prepareMinecraft(ctx, profile.GameVersion); err != nil {
		return PrepareClientResult{}, err
	}
	if err := downloader.prepareLoader(ctx, profile); err != nil {
		return PrepareClientResult{}, err
	}

	// Собираем настоящую команду запуска из version JSON (с учётом загрузчика)
	// и сохраняем её в профиль до индексации файлов.
	if err := s.buildAndSaveLaunchCommands(&profile); err != nil {
		return PrepareClientResult{}, err
	}
	if err := s.db.WithContext(ctx).Model(&profile).
		Select("launch_command_windows", "launch_command_linux", "launch_command_mac_os").
		Updates(map[string]interface{}{
			"launch_command_windows": profile.LaunchCommandWindows,
			"launch_command_linux":   profile.LaunchCommandLinux,
			"launch_command_mac_os":  profile.LaunchCommandMacOS,
		}).Error; err != nil {
		return PrepareClientResult{}, err
	}

	scan, err := s.Scan(ctx, id)
	if err != nil {
		return PrepareClientResult{}, err
	}

	return PrepareClientResult{
		Profile:    scan.Profile,
		FileCount:  scan.FileCount,
		TotalSize:  scan.TotalSize,
		Downloaded: downloader.downloaded,
		Message:    "Клиент подготовлен. Manifest пересобран.",
	}, nil
}

type profileDownloader struct {
	client     http.Client
	root       string
	downloaded int
}

func (d *profileDownloader) prepareMinecraft(ctx context.Context, gameVersion string) error {
	versionURL, err := d.minecraftVersionURL(ctx, gameVersion)
	if err != nil {
		return err
	}

	var version minecraftVersion
	versionBytes, err := d.getBytes(ctx, versionURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(versionBytes, &version); err != nil {
		return err
	}
	if version.ID == "" {
		version.ID = gameVersion
	}

	versionDir := filepath.Join("versions", version.ID)
	if err := d.writeBytes(filepath.Join(versionDir, version.ID+".json"), versionBytes, ""); err != nil {
		return err
	}
	if version.Downloads.Client.URL == "" {
		return errors.New("minecraft version has no client download")
	}
	if err := d.download(filepath.Join(versionDir, version.ID+".jar"), version.Downloads.Client.URL, version.Downloads.Client.SHA1); err != nil {
		return err
	}

	for _, library := range version.Libraries {
		if library.Downloads.Artifact != nil && library.Downloads.Artifact.URL != "" && library.Downloads.Artifact.Path != "" {
			if err := d.download(filepath.Join("libraries", filepath.FromSlash(library.Downloads.Artifact.Path)), library.Downloads.Artifact.URL, library.Downloads.Artifact.SHA1); err != nil {
				return err
			}
		}
		for _, classifier := range library.Downloads.Classifiers {
			if classifier.URL == "" || classifier.Path == "" {
				continue
			}
			if err := d.download(filepath.Join("libraries", filepath.FromSlash(classifier.Path)), classifier.URL, classifier.SHA1); err != nil {
				return err
			}
		}
	}

	if version.AssetIndex.URL != "" && version.AssetIndex.ID != "" {
		assetBytes, err := d.getBytes(ctx, version.AssetIndex.URL)
		if err != nil {
			return err
		}
		if err := d.writeBytes(filepath.Join("assets", "indexes", version.AssetIndex.ID+".json"), assetBytes, version.AssetIndex.SHA1); err != nil {
			return err
		}
		var assets assetIndex
		if err := json.Unmarshal(assetBytes, &assets); err != nil {
			return err
		}
		for _, asset := range assets.Objects {
			if len(asset.Hash) < 2 {
				continue
			}
			objectPath := filepath.Join("assets", "objects", asset.Hash[:2], asset.Hash)
			objectURL := "https://resources.download.minecraft.net/" + asset.Hash[:2] + "/" + asset.Hash
			if err := d.download(objectPath, objectURL, asset.Hash); err != nil {
				return err
			}
		}
	}

	if version.Logging.Client.File.URL != "" {
		logFile := version.Logging.Client.File
		fileName := logFile.ID
		if fileName == "" {
			fileName = filepath.Base(logFile.Path)
		}
		if fileName != "" {
			if err := d.download(filepath.Join("assets", "log_configs", fileName), logFile.URL, logFile.SHA1); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *profileDownloader) prepareLoader(ctx context.Context, profile models.Profile) error {
	switch strings.ToLower(profile.Loader) {
	case "", "vanilla":
		return nil
	case "fabric":
		return d.prepareFabricLike(ctx, "fabric", "https://meta.fabricmc.net/v2/versions/loader/%s/%s/profile/json", profile.GameVersion, profile.LoaderVersion)
	case "quilt":
		return d.prepareFabricLike(ctx, "quilt", "https://meta.quiltmc.org/v3/versions/loader/%s/%s/profile/json", profile.GameVersion, profile.LoaderVersion)
	case "forge":
		if profile.LoaderVersion == "" {
			return errors.New("выберите версию Forge")
		}
		fullVersion := profile.GameVersion + "-" + profile.LoaderVersion
		installerRel := filepath.Join("installers", "forge-"+fullVersion+"-installer.jar")
		url := "https://maven.minecraftforge.net/net/minecraftforge/forge/" + fullVersion + "/forge-" + fullVersion + "-installer.jar"
		if err := d.downloadWithRemoteSHA1(ctx, installerRel, url); err != nil {
			return err
		}
		return d.runInstaller(installerRel)
	case "neoforge":
		artifact, err := neoForgeInstallerArtifact(profile)
		if err != nil {
			return err
		}
		installerRel := filepath.Join("installers", artifact.fileName)
		if err := d.downloadWithRemoteSHA1(ctx, installerRel, artifact.url); err != nil {
			return err
		}
		return d.runInstaller(installerRel)
	default:
		return fmt.Errorf("неизвестный загрузчик: %s", profile.Loader)
	}
}

// runInstaller запускает официальный установщик Forge/NeoForge в headless-режиме
// (--install-client). Он обрабатывает процессоры (патч клиентского jar, библиотеки)
// и создаёт versions/<id>/<id>.json — без этого мод-загрузчик не запустится.
func (d *profileDownloader) runInstaller(installerRel string) error {
	// Установщику нужен абсолютный путь к каталогу установки, иначе он ищет
	// Minecraft относительно собственного рабочего каталога.
	root, err := filepath.Abs(d.root)
	if err != nil {
		return err
	}
	installerPath, err := safeJoin(d.root, filepath.ToSlash(installerRel))
	if err != nil {
		return err
	}
	if installerPath, err = filepath.Abs(installerPath); err != nil {
		return err
	}

	// Установщику Forge/NeoForge нужен launcher_profiles.json в целевом каталоге.
	profilesPath := filepath.Join(root, "launcher_profiles.json")
	if err := os.WriteFile(profilesPath, []byte(`{"profiles":{},"selectedProfile":"","clientToken":""}`), 0644); err != nil {
		return fmt.Errorf("не удалось подготовить launcher_profiles.json: %w", err)
	}

	cmd := exec.Command(javaBinary(), "-jar", installerPath, "--install-client", root)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()

	// Чистим артефакты установщика, чтобы они не попадали в манифест/скачивание клиентом.
	os.Remove(profilesPath)
	os.Remove(filepath.Join(root, "installer.log"))
	os.RemoveAll(filepath.Join(root, "installers"))

	if err != nil {
		return fmt.Errorf("установщик загрузчика завершился с ошибкой: %w\n%s", err, string(output))
	}
	return nil
}

func javaBinary() string {
	if bin := strings.TrimSpace(os.Getenv("LAUNCHER_JAVA_BIN")); bin != "" {
		return bin
	}
	return "java"
}

func (d *profileDownloader) prepareFabricLike(ctx context.Context, name, endpoint, gameVersion, loaderVersion string) error {
	if loaderVersion == "" {
		return fmt.Errorf("выберите версию %s", name)
	}

	profileURL := fmt.Sprintf(endpoint, gameVersion, loaderVersion)
	profileBytes, err := d.getBytes(ctx, profileURL)
	if err != nil {
		return err
	}

	var loader loaderProfile
	if err := json.Unmarshal(profileBytes, &loader); err != nil {
		return err
	}
	if loader.ID == "" {
		loader.ID = name + "-" + loaderVersion + "-" + gameVersion
	}
	if err := d.writeBytes(filepath.Join("versions", loader.ID, loader.ID+".json"), profileBytes, ""); err != nil {
		return err
	}

	for _, library := range loader.Libraries {
		path, err := mavenArtifactPath(library.Name)
		if err != nil || library.URL == "" {
			continue
		}
		if err := d.download(filepath.Join("libraries", filepath.FromSlash(path)), strings.TrimRight(library.URL, "/")+"/"+path, library.SHA1); err != nil {
			return err
		}
	}

	return nil
}

func neoForgeInstallerArtifact(profile models.Profile) (installerArtifact, error) {
	loaderVersion := strings.TrimSpace(profile.LoaderVersion)
	if loaderVersion == "" {
		return installerArtifact{}, errors.New("выберите версию NeoForge")
	}

	if strings.HasPrefix(loaderVersion, profile.GameVersion+"-") {
		fileName := "neoforge-legacy-" + loaderVersion + "-installer.jar"
		url := "https://maven.neoforged.net/releases/net/neoforged/forge/" + loaderVersion + "/forge-" + loaderVersion + "-installer.jar"
		return installerArtifact{fileName: fileName, url: url}, nil
	}

	prefix := neoForgePrefix(profile.GameVersion)
	if prefix != "" && !strings.HasPrefix(loaderVersion, prefix) {
		loaderVersion = prefix + loaderVersion
	}
	fileName := "neoforge-" + loaderVersion + "-installer.jar"
	url := "https://maven.neoforged.net/releases/net/neoforged/neoforge/" + loaderVersion + "/neoforge-" + loaderVersion + "-installer.jar"
	return installerArtifact{fileName: fileName, url: url}, nil
}

func (d *profileDownloader) minecraftVersionURL(ctx context.Context, gameVersion string) (string, error) {
	var manifest versionManifest
	data, err := d.getBytes(ctx, "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json")
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", err
	}
	for _, version := range manifest.Versions {
		if version.ID == gameVersion {
			return version.URL, nil
		}
	}
	return "", fmt.Errorf("версия Minecraft не найдена: %s", gameVersion)
}

func (d *profileDownloader) getBytes(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := d.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("download failed: HTTP %d %s", response.StatusCode, endpoint)
	}
	return io.ReadAll(response.Body)
}

func (d *profileDownloader) download(relPath, endpoint, expectedSHA1 string) error {
	target, err := safeJoin(d.root, filepath.ToSlash(relPath))
	if err != nil {
		return err
	}
	if fileMatchesSHA1(target, expectedSHA1) {
		return nil
	}

	data, err := d.getBytes(context.Background(), endpoint)
	if err != nil {
		return err
	}
	return d.writeBytes(relPath, data, expectedSHA1)
}

func (d *profileDownloader) downloadWithRemoteSHA1(ctx context.Context, relPath, endpoint string) error {
	expectedSHA1 := ""
	data, err := d.getBytes(ctx, endpoint+".sha1")
	if err == nil {
		expectedSHA1 = firstField(string(data))
	}
	return d.download(relPath, endpoint, expectedSHA1)
}

func firstField(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (d *profileDownloader) writeBytes(relPath string, data []byte, expectedSHA1 string) error {
	if expectedSHA1 != "" && sha1Hex(data) != strings.ToLower(expectedSHA1) {
		return fmt.Errorf("sha1 mismatch: %s", relPath)
	}

	target, err := safeJoin(d.root, filepath.ToSlash(relPath))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return err
	}
	d.downloaded++
	return nil
}

func fileMatchesSHA1(path string, expectedSHA1 string) bool {
	if expectedSHA1 == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return sha1Hex(data) == strings.ToLower(expectedSHA1)
}

func sha1Hex(data []byte) string {
	hash := sha1.Sum(data)
	return hex.EncodeToString(hash[:])
}

func mavenArtifactPath(name string) (string, error) {
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid maven coordinate: %s", name)
	}
	group := strings.ReplaceAll(parts[0], ".", "/")
	artifact := parts[1]
	version := parts[2]
	classifier := ""
	if len(parts) >= 4 {
		classifier = "-" + parts[3]
	}
	file := artifact + "-" + version + classifier + ".jar"
	return group + "/" + artifact + "/" + version + "/" + file, nil
}
