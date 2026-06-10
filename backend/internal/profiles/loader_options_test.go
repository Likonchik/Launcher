package profiles

import (
	"reflect"
	"testing"

	"launcher-backend/internal/models"
)

func TestLatestFirstUsesNumericVersionOrder(t *testing.T) {
	got := latestFirst([]string{
		"21.1.99",
		"21.1.233",
		"21.1.9",
		"21.1.233-beta",
		"21.1.100",
	})
	want := []string{
		"21.1.233",
		"21.1.233-beta",
		"21.1.100",
		"21.1.99",
		"21.1.9",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("latestFirst() = %#v, want %#v", got, want)
	}
}

func TestNeoForgeLoaderVersionsForMinecraft1211UseNewestFirst(t *testing.T) {
	got := neoForgeLoaderVersions([]string{
		"20.4.250",
		"21.0.167",
		"21.1.99",
		"21.1.224",
		"21.1.226",
		"21.1.225",
		"21.1.233",
		"22.0.1",
	}, "21.1.")
	want := []LoaderVersion{
		{Value: "21.1.233", Label: "21.1.233", Stable: true},
		{Value: "21.1.226", Label: "21.1.226", Stable: true},
		{Value: "21.1.225", Label: "21.1.225", Stable: true},
		{Value: "21.1.224", Label: "21.1.224", Stable: true},
		{Value: "21.1.99", Label: "21.1.99", Stable: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("neoForgeLoaderVersions() = %#v, want %#v", got, want)
	}
}

func TestNeoForgeInstallerArtifactExpandsShortVersion(t *testing.T) {
	artifact, err := neoForgeInstallerArtifact(models.Profile{
		GameVersion:   "1.21.1",
		LoaderVersion: "233",
	})
	if err != nil {
		t.Fatalf("neoForgeInstallerArtifact() error = %v", err)
	}
	if artifact.fileName != "neoforge-21.1.233-installer.jar" {
		t.Fatalf("fileName = %q", artifact.fileName)
	}
	wantURL := "https://maven.neoforged.net/releases/net/neoforged/neoforge/21.1.233/neoforge-21.1.233-installer.jar"
	if artifact.url != wantURL {
		t.Fatalf("url = %q, want %q", artifact.url, wantURL)
	}
}

func TestNeoForgeInstallerArtifactKeepsLegacyVersion(t *testing.T) {
	artifact, err := neoForgeInstallerArtifact(models.Profile{
		GameVersion:   "1.20.1",
		LoaderVersion: "1.20.1-47.1.106",
	})
	if err != nil {
		t.Fatalf("neoForgeInstallerArtifact() error = %v", err)
	}
	if artifact.fileName != "neoforge-legacy-1.20.1-47.1.106-installer.jar" {
		t.Fatalf("fileName = %q", artifact.fileName)
	}
}
