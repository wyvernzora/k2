package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wyvernzora/k2/kairos/image-build/internal/plan"
	"github.com/wyvernzora/k2/kairos/image-build/internal/rawpatch"
)

type Builder struct {
	Stdout  io.Writer
	Stderr  io.Writer
	Runner  CommandRunner
	Patcher RawPatcher
}

type CommandRunner interface {
	Run(name string, args []string, stdout io.Writer, stderr io.Writer) error
}

type RawPatcher interface {
	Patch(rawFile string, resolved plan.Plan) error
}

type ExecRunner struct{}

type manifest struct {
	Target            string          `json:"target"`
	Image             string          `json:"image"`
	KairosVersion     string          `json:"kairosVersion"`
	KairosInitVersion string          `json:"kairosInitVersion"`
	AuroraBootVersion string          `json:"aurorabootVersion"`
	K3sVersion        string          `json:"k3sVersion"`
	Raw               manifestFile    `json:"raw"`
	Compressed        manifestFile    `json:"compressed"`
	Patches           manifestPatches `json:"patches"`
}

type manifestFile struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type manifestPatches struct {
	Raw []plan.RawPatch `json:"raw"`
}

func (ExecRunner) Run(name string, args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (b Builder) Artifact(resolved plan.Plan) error {
	if b.Stdout == nil {
		b.Stdout = io.Discard
	}
	if b.Stderr == nil {
		b.Stderr = io.Discard
	}
	if b.Runner == nil {
		b.Runner = ExecRunner{}
	}
	if b.Patcher == nil {
		b.Patcher = rawpatch.Patcher{
			Stdout: b.Stdout,
			Stderr: b.Stderr,
		}
	}

	if err := os.MkdirAll(resolved.ArtifactDir, 0o755); err != nil {
		return err
	}
	if err := cleanArtifacts(resolved.ArtifactDir); err != nil {
		return err
	}

	for _, kind := range resolved.Artifacts {
		switch kind {
		case "raw":
			if err := b.buildRaw(resolved); err != nil {
				return err
			}
		case "iso":
			if err := b.buildISO(resolved); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported artifact type %q", kind)
		}
	}

	if err := WriteChecksums(resolved.ArtifactDir); err != nil {
		return err
	}
	if err := WriteManifest(resolved); err != nil {
		return err
	}

	fmt.Fprintf(b.Stdout, "Artifacts written to %s\n", resolved.ArtifactDir)
	return nil
}

func (b Builder) buildRaw(resolved plan.Plan) error {
	if resolved.ArtifactOptions.Raw.DiskStateSize == nil {
		return fmt.Errorf("raw artifact for %s requires diskStateSize", resolved.Target)
	}

	args := []string{
		"run",
		"--rm",
		"--privileged",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", resolved.ArtifactDir + ":/output",
		"quay.io/kairos/auroraboot:" + resolved.Versions.AuroraBootVersion,
		"--debug",
		"--set", "disable_http_server=true",
		"--set", "disable_netboot=true",
		"--set", "arch=" + resolved.Arch,
		"--set", "container_image=docker:" + resolved.Image,
		"--set", "state_dir=/output",
		"--set", "disk.raw=true",
		"--set", fmt.Sprintf("disk.state_size=%d", *resolved.ArtifactOptions.Raw.DiskStateSize),
	}
	if err := b.Runner.Run("docker", args, b.Stdout, b.Stderr); err != nil {
		return fmt.Errorf("auroraboot raw build failed: %w", err)
	}

	rawFile, err := singleGeneratedFile(resolved.ArtifactDir, "*.raw")
	if err != nil {
		return err
	}
	expectedRaw := filepath.Join(resolved.ArtifactDir, resolved.ArtifactStem+".raw")
	if rawFile != expectedRaw {
		if err := os.Rename(rawFile, expectedRaw); err != nil {
			return err
		}
	}

	if err := b.Patcher.Patch(expectedRaw, resolved); err != nil {
		return fmt.Errorf("raw artifact patching failed: %w", err)
	}
	if err := b.Runner.Run("xz", []string{"-T0", "-zkf", expectedRaw}, b.Stdout, b.Stderr); err != nil {
		return fmt.Errorf("compressing raw artifact failed: %w", err)
	}
	return nil
}

func (b Builder) buildISO(resolved plan.Plan) error {
	args := []string{
		"run",
		"--rm",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", resolved.ArtifactDir + ":/output",
		"quay.io/kairos/auroraboot:" + resolved.Versions.AuroraBootVersion,
		"--debug",
		"build-iso",
		"--output", "/output/",
		"docker:" + resolved.Image,
	}
	if err := b.Runner.Run("docker", args, b.Stdout, b.Stderr); err != nil {
		return fmt.Errorf("auroraboot ISO build failed: %w", err)
	}

	isoFile, err := singleGeneratedFile(resolved.ArtifactDir, "*.iso")
	if err != nil {
		return err
	}
	expectedISO := filepath.Join(resolved.ArtifactDir, resolved.ArtifactStem+".iso")
	if isoFile != expectedISO {
		if err := os.Rename(isoFile, expectedISO); err != nil {
			return err
		}
	}
	return nil
}

func WriteChecksums(dir string) error {
	files, err := checksumFiles(dir)
	if err != nil {
		return err
	}

	out, err := os.Create(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		return err
	}
	defer out.Close()

	for _, file := range files {
		hash, err := SHA256File(file)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s  %s\n", hash, filepath.Base(file)); err != nil {
			return err
		}
	}
	return nil
}

func WriteManifest(resolved plan.Plan) error {
	rawName, rawSHA, err := optionalManifestFile(resolved.ArtifactDir, "*.raw")
	if err != nil {
		return err
	}
	compressedName, compressedSHA, err := optionalManifestFile(resolved.ArtifactDir, "*.raw.xz")
	if err != nil {
		return err
	}

	doc := manifest{
		Target:            resolved.Target,
		Image:             resolved.Image,
		KairosVersion:     resolved.Versions.KairosVersion,
		KairosInitVersion: resolved.Versions.KairosInitVersion,
		AuroraBootVersion: resolved.Versions.AuroraBootVersion,
		K3sVersion:        resolved.Versions.K3sVersion,
		Raw: manifestFile{
			File:   rawName,
			SHA256: rawSHA,
		},
		Compressed: manifestFile{
			File:   compressedName,
			SHA256: compressedSHA,
		},
		Patches: manifestPatches{
			Raw: append([]plan.RawPatch(nil), resolved.RawPatches...),
		},
	}

	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(filepath.Join(resolved.ArtifactDir, "artifact-manifest.json"), encoded, 0o644)
}

func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func cleanArtifacts(dir string) error {
	patterns := []string{"*.raw", "*.raw.xz", "*.iso", "SHA256SUMS", "artifact-manifest.json"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return err
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			if info.IsDir() {
				continue
			}
			if err := os.Remove(match); err != nil {
				return err
			}
		}
	}
	return nil
}

func checksumFiles(dir string) ([]string, error) {
	patterns := []string{"*.raw", "*.raw.xz", "*.iso"}
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return nil, err
			}
			if !info.IsDir() {
				files = append(files, match)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func optionalManifestFile(dir string, pattern string) (string, string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return "", "", err
	}
	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return "", "", err
		}
		if !info.IsDir() {
			files = append(files, match)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return "", "", nil
	}
	if len(files) > 1 {
		return "", "", fmt.Errorf("expected at most one %s file in %s, found %d", pattern, dir, len(files))
	}
	hash, err := SHA256File(files[0])
	if err != nil {
		return "", "", err
	}
	return filepath.Base(files[0]), hash, nil
}

func singleGeneratedFile(dir string, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return "", err
	}
	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			files = append(files, match)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return "", fmt.Errorf("auroraboot did not produce a %s artifact in %s", strings.TrimPrefix(pattern, "*"), dir)
	}
	if len(files) > 1 {
		return "", fmt.Errorf("expected one generated %s artifact in %s, found %d", pattern, dir, len(files))
	}
	return files[0], nil
}
