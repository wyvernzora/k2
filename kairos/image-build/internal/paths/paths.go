package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	BuildRoot      string `json:"buildRoot" yaml:"buildRoot"`
	KairosRoot     string `json:"kairosRoot" yaml:"kairosRoot"`
	TargetsFile    string `json:"targetsFile" yaml:"targetsFile"`
	VersionsFile   string `json:"versionsFile" yaml:"versionsFile"`
	OverlaysDir    string `json:"overlaysDir" yaml:"overlaysDir"`
	ArtifactsDir   string `json:"artifactsDir" yaml:"artifactsDir"`
	DockerfilePath string `json:"dockerfilePath" yaml:"dockerfilePath"`
}

type Overrides struct {
	BuildRoot    string
	KairosRoot   string
	TargetsFile  string
	VersionsFile string
	OverlaysDir  string
	ArtifactsDir string
}

func Discover(cwd string, overrides Overrides) (Paths, error) {
	buildRoot := overrides.BuildRoot
	if buildRoot == "" {
		var err error
		buildRoot, err = discoverBuildRoot(cwd)
		if err != nil {
			return Paths{}, err
		}
	}
	buildRoot, err := filepath.Abs(buildRoot)
	if err != nil {
		return Paths{}, err
	}

	kairosRoot := overrides.KairosRoot
	if kairosRoot == "" {
		kairosRoot = filepath.Join(buildRoot, "..")
	}
	kairosRoot, err = filepath.Abs(kairosRoot)
	if err != nil {
		return Paths{}, err
	}

	targetsFile := defaultPath(overrides.TargetsFile, kairosRoot, "targets.yaml")
	versionsFile := defaultPath(overrides.VersionsFile, kairosRoot, "versions.env")
	overlaysDir := defaultPath(overrides.OverlaysDir, kairosRoot, "overlays")
	artifactsDir := defaultPath(overrides.ArtifactsDir, kairosRoot, "artifacts")

	return Paths{
		BuildRoot:      buildRoot,
		KairosRoot:     kairosRoot,
		TargetsFile:    targetsFile,
		VersionsFile:   versionsFile,
		OverlaysDir:    overlaysDir,
		ArtifactsDir:   artifactsDir,
		DockerfilePath: filepath.Join(buildRoot, "Dockerfile"),
	}, nil
}

func discoverBuildRoot(cwd string) (string, error) {
	if candidate := filepath.Join(cwd, "kairos", "image-build"); fileExists(filepath.Join(candidate, "go.mod")) {
		return candidate, nil
	}

	dir := cwd
	for {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}

	return "", fmt.Errorf("unable to discover image-build root from %s; pass --build-root", cwd)
}

func defaultPath(value, root, name string) string {
	if value != "" {
		abs, err := filepath.Abs(value)
		if err == nil {
			return abs
		}
		return value
	}
	return filepath.Join(root, name)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
