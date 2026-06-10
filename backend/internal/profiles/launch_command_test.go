package profiles

import (
	"strings"
	"testing"
)

func TestBuildClasspathSkipsLibrariesAlreadyOnModulePath(t *testing.T) {
	target := launchTargets["linux"]
	version := versionJSON{
		Libraries: []versionLib{
			testVersionLib("cpw/mods/bootstraplauncher/2.0.2/bootstraplauncher-2.0.2.jar"),
			testVersionLib("cpw/mods/securejarhandler/3.0.8/securejarhandler-3.0.8.jar"),
			testVersionLib("com/google/code/gson/gson/2.10.1/gson-2.10.1.jar"),
		},
	}
	excluded := modulePathClasspathExclusions([]string{
		"-p",
		"libraries/cpw/mods/bootstraplauncher/2.0.2/bootstraplauncher-2.0.2.jar" +
			target.separator +
			"libraries/cpw/mods/securejarhandler/3.0.8/securejarhandler-3.0.8.jar",
	}, target.separator)

	classpath := buildClasspath(
		version,
		"libraries/net/minecraft/client/1.21.1-20240808.144430/client-1.21.1-20240808.144430-extra.jar",
		target,
		excluded,
	)

	if strings.Contains(classpath, "bootstraplauncher") {
		t.Fatalf("classpath contains bootstraplauncher: %s", classpath)
	}
	if strings.Contains(classpath, "securejarhandler") {
		t.Fatalf("classpath contains securejarhandler: %s", classpath)
	}
	if !strings.Contains(classpath, "gson-2.10.1.jar") {
		t.Fatalf("classpath does not contain regular library: %s", classpath)
	}
	if !strings.Contains(classpath, "client-1.21.1-20240808.144430-extra.jar") {
		t.Fatalf("classpath does not contain client-extra: %s", classpath)
	}
}

func testVersionLib(path string) versionLib {
	downloads := struct {
		Artifact *artifactInfo `json:"artifact"`
	}{
		Artifact: &artifactInfo{Path: path},
	}
	return versionLib{Downloads: &downloads}
}
