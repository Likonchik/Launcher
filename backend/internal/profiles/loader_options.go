package profiles

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LoaderOptions struct {
	MinecraftVersions []string       `json:"minecraftVersions"`
	Loaders           []LoaderOption `json:"loaders"`
}

type LoaderOption struct {
	ID              string          `json:"id"`
	Label           string          `json:"label"`
	JavaVersion     int             `json:"javaVersion"`
	RequiresVersion bool            `json:"requiresVersion"`
	Versions        []LoaderVersion `json:"versions"`
}

type LoaderVersion struct {
	Value  string `json:"value"`
	Label  string `json:"label"`
	Stable bool   `json:"stable"`
}

func (s Service) LoaderOptions(ctx context.Context, gameVersion string) LoaderOptions {
	gameVersion = strings.TrimSpace(gameVersion)
	if gameVersion == "" {
		gameVersion = "1.21.1"
	}

	loaders := []LoaderOption{{
		ID:              "vanilla",
		Label:           "Vanilla",
		JavaVersion:     javaVersionForMinecraft(gameVersion),
		RequiresVersion: false,
		Versions: []LoaderVersion{{
			Value:  "",
			Label:  "Без загрузчика",
			Stable: true,
		}},
	}}

	for _, loader := range []LoaderOption{
		{
			ID:              "fabric",
			Label:           "Fabric",
			JavaVersion:     javaVersionForMinecraft(gameVersion),
			RequiresVersion: true,
			Versions:        fabricVersions(ctx, gameVersion),
		},
		{
			ID:              "forge",
			Label:           "Forge",
			JavaVersion:     javaVersionForMinecraft(gameVersion),
			RequiresVersion: true,
			Versions:        forgeVersions(ctx, gameVersion),
		},
		{
			ID:              "neoforge",
			Label:           "NeoForge",
			JavaVersion:     javaVersionForMinecraft(gameVersion),
			RequiresVersion: true,
			Versions:        neoforgeVersions(ctx, gameVersion),
		},
		{
			ID:              "quilt",
			Label:           "Quilt",
			JavaVersion:     javaVersionForMinecraft(gameVersion),
			RequiresVersion: true,
			Versions:        quiltVersions(ctx, gameVersion),
		},
	} {
		if len(loader.Versions) > 0 {
			loaders = append(loaders, loader)
		}
	}

	return LoaderOptions{
		MinecraftVersions: commonMinecraftVersions(),
		Loaders:           loaders,
	}
}

func fabricVersions(ctx context.Context, gameVersion string) []LoaderVersion {
	type fabricEntry struct {
		Loader struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		} `json:"loader"`
	}

	var entries []fabricEntry
	if !getJSON(ctx, "https://meta.fabricmc.net/v2/versions/loader/"+gameVersion, &entries) {
		return nil
	}

	versions := make([]LoaderVersion, 0, len(entries))
	for _, entry := range entries {
		if entry.Loader.Version == "" {
			continue
		}
		versions = append(versions, LoaderVersion{
			Value:  entry.Loader.Version,
			Label:  versionLabel(entry.Loader.Version, entry.Loader.Stable),
			Stable: entry.Loader.Stable,
		})
	}
	return limitVersions(versions, 25)
}

func quiltVersions(ctx context.Context, gameVersion string) []LoaderVersion {
	type quiltEntry struct {
		Loader struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		} `json:"loader"`
	}

	var entries []quiltEntry
	if !getJSON(ctx, "https://meta.quiltmc.org/v3/versions/loader/"+gameVersion, &entries) {
		return nil
	}

	versions := make([]LoaderVersion, 0, len(entries))
	for _, entry := range entries {
		if entry.Loader.Version == "" {
			continue
		}
		versions = append(versions, LoaderVersion{
			Value:  entry.Loader.Version,
			Label:  versionLabel(entry.Loader.Version, entry.Loader.Stable),
			Stable: entry.Loader.Stable,
		})
	}
	return limitVersions(versions, 25)
}

func forgeVersions(ctx context.Context, gameVersion string) []LoaderVersion {
	versions := mavenVersions(ctx, "https://maven.minecraftforge.net/net/minecraftforge/forge/maven-metadata.xml")
	prefix := gameVersion + "-"
	result := make([]LoaderVersion, 0)
	for _, version := range latestFirst(versions) {
		if strings.HasPrefix(version, prefix) {
			value := strings.TrimPrefix(version, prefix)
			result = append(result, LoaderVersion{
				Value:  value,
				Label:  value,
				Stable: true,
			})
		}
	}
	return limitVersions(result, 25)
}

func neoforgeVersions(ctx context.Context, gameVersion string) []LoaderVersion {
	if gameVersion == "1.20.1" {
		return legacyNeoForgeVersions(ctx, gameVersion)
	}

	versions := mavenVersions(ctx, "https://maven.neoforged.net/releases/net/neoforged/neoforge/maven-metadata.xml")
	prefix := neoForgePrefix(gameVersion)
	if prefix == "" {
		return nil
	}
	return neoForgeLoaderVersions(versions, prefix)
}

func neoForgeLoaderVersions(versions []string, prefix string) []LoaderVersion {
	result := make([]LoaderVersion, 0)
	for _, version := range latestFirst(versions) {
		if strings.HasPrefix(version, prefix) {
			result = append(result, LoaderVersion{
				Value:  version,
				Label:  version,
				Stable: true,
			})
		}
	}
	return limitVersions(result, 25)
}

func legacyNeoForgeVersions(ctx context.Context, gameVersion string) []LoaderVersion {
	versions := mavenVersions(ctx, "https://maven.neoforged.net/releases/net/neoforged/forge/maven-metadata.xml")
	prefix := gameVersion + "-"
	result := make([]LoaderVersion, 0)
	for _, version := range latestFirst(versions) {
		if strings.HasPrefix(version, prefix) {
			result = append(result, LoaderVersion{
				Value:  version,
				Label:  strings.TrimPrefix(version, prefix) + " legacy",
				Stable: true,
			})
		}
	}
	return limitVersions(result, 25)
}

func mavenVersions(ctx context.Context, endpoint string) []string {
	type metadata struct {
		Versioning struct {
			Versions []string `xml:"versions>version"`
		} `xml:"versioning"`
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	client := http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil
	}

	var data metadata
	if err := xml.NewDecoder(response.Body).Decode(&data); err != nil {
		return nil
	}
	return data.Versioning.Versions
}

func getJSON(ctx context.Context, endpoint string, target any) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	client := http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return false
	}
	return json.NewDecoder(response.Body).Decode(target) == nil
}

func commonMinecraftVersions() []string {
	return []string{
		"1.21.1",
		"1.21",
		"1.20.6",
		"1.20.4",
		"1.20.1",
		"1.19.4",
		"1.19.2",
		"1.18.2",
		"1.17.1",
		"1.16.5",
		"1.12.2",
		"1.7.10",
	}
}

func javaVersionForMinecraft(gameVersion string) int {
	switch {
	case strings.HasPrefix(gameVersion, "1.21"):
		return 21
	case strings.HasPrefix(gameVersion, "1.20.5"), strings.HasPrefix(gameVersion, "1.20.6"):
		return 21
	case strings.HasPrefix(gameVersion, "1.18"),
		strings.HasPrefix(gameVersion, "1.19"),
		strings.HasPrefix(gameVersion, "1.20"):
		return 17
	case strings.HasPrefix(gameVersion, "1.17"):
		return 16
	default:
		return 8
	}
}

func neoForgePrefix(gameVersion string) string {
	parts := strings.Split(gameVersion, ".")
	if len(parts) < 2 || parts[0] != "1" {
		return ""
	}
	minor := parts[1]
	patch := "0"
	if len(parts) >= 3 {
		patch = parts[2]
	}
	return minor + "." + patch + "."
}

func latestFirst(values []string) []string {
	result := append([]string(nil), values...)
	sort.Slice(result, func(i, j int) bool {
		return compareMavenVersions(result[i], result[j]) > 0
	})
	return result
}

func compareMavenVersions(left, right string) int {
	leftParts := versionParts(left)
	rightParts := versionParts(right)
	limit := len(leftParts)
	if len(rightParts) > limit {
		limit = len(rightParts)
	}

	for index := 0; index < limit; index++ {
		if index >= len(leftParts) {
			if remainingIsPreRelease(rightParts[index:]) {
				return 1
			}
			return -1
		}
		if index >= len(rightParts) {
			if remainingIsPreRelease(leftParts[index:]) {
				return -1
			}
			return 1
		}

		leftNumber, leftIsNumber := parseVersionNumber(leftParts[index])
		rightNumber, rightIsNumber := parseVersionNumber(rightParts[index])
		switch {
		case leftIsNumber && rightIsNumber:
			if leftNumber > rightNumber {
				return 1
			}
			if leftNumber < rightNumber {
				return -1
			}
		case leftIsNumber:
			return 1
		case rightIsNumber:
			return -1
		default:
			if leftParts[index] > rightParts[index] {
				return 1
			}
			if leftParts[index] < rightParts[index] {
				return -1
			}
		}
	}
	return 0
}

func versionParts(value string) []string {
	return strings.FieldsFunc(strings.ToLower(value), func(char rune) bool {
		return char == '.' || char == '-' || char == '_' || char == '+'
	})
}

func parseVersionNumber(value string) (int, bool) {
	number, err := strconv.Atoi(value)
	return number, err == nil
}

func remainingIsPreRelease(parts []string) bool {
	for _, part := range parts {
		if _, ok := parseVersionNumber(part); ok {
			return false
		}
		switch part {
		case "alpha", "beta", "pre", "preview", "rc", "snapshot":
			continue
		default:
			return false
		}
	}
	return len(parts) > 0
}

func versionLabel(version string, stable bool) string {
	if stable {
		return version + " stable"
	}
	return version
}

func limitVersions(values []LoaderVersion, limit int) []LoaderVersion {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
