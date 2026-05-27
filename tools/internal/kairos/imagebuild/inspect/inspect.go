package inspect

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/plan"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/rawpatch"
)

type Inspector struct {
	Context context.Context
	Stdout  io.Writer
	Stderr  io.Writer
	Runner  CommandRunner
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, stdout io.Writer, stderr io.Writer) error
}

type ExecRunner struct{}

type rawFiles struct {
	Raw        string
	Compressed string
}

func (ExecRunner) Run(ctx context.Context, name string, args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (i Inspector) Artifact(resolved plan.Plan) error {
	i = i.withDefaults()
	if i.Stdout == nil {
		i.Stdout = io.Discard
	}

	if info, err := os.Stat(resolved.ArtifactDir); err != nil {
		return fmt.Errorf("missing artifact directory %s: %w", resolved.ArtifactDir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("artifact path %s is not a directory", resolved.ArtifactDir)
	}

	expectedFiles := []string{}
	for _, artifact := range resolved.Artifacts {
		switch artifact {
		case "raw":
			files, err := i.inspectRaw(resolved)
			if err != nil {
				return err
			}
			expectedFiles = append(expectedFiles, files.Compressed)
		case "iso":
			iso, err := i.inspectISO(resolved)
			if err != nil {
				return err
			}
			expectedFiles = append(expectedFiles, iso)
		default:
			return fmt.Errorf("unsupported artifact type %q", artifact)
		}
	}

	if err := i.verifyChecksums(resolved.ArtifactDir, expectedFiles); err != nil {
		return err
	}

	fmt.Fprintf(i.Stdout, "Artifact inspection passed for %s\n", resolved.Target)
	return nil
}

func (i Inspector) inspectRaw(resolved plan.Plan) (rawFiles, error) {
	files := RawFiles(resolved)
	if err := requireFile(files.Compressed); err != nil {
		return rawFiles{}, err
	}

	requests, err := rawExtractRequests(resolved)
	if err != nil {
		return rawFiles{}, err
	}
	if len(requests) > 0 {
		workDir, err := os.MkdirTemp("", "k2-kairos-inspect-*")
		if err != nil {
			return rawFiles{}, err
		}
		defer os.RemoveAll(workDir)

		rawForInspection, err := i.rawForInspection(files, workDir)
		if err != nil {
			return rawFiles{}, err
		}

		if err := rawpatch.ExtractFilesContext(i.Context, rawForInspection, resolved.Versions.AuroraBootVersion, requests, workDir); err != nil {
			return rawFiles{}, err
		}
		if len(resolved.RawPatches) > 0 {
			if err := validateRawPatches(workDir, resolved); err != nil {
				return rawFiles{}, err
			}
		}
		if err := validateRawExpectations(workDir, resolved); err != nil {
			return rawFiles{}, err
		}
	}

	return files, nil
}

func (i Inspector) rawForInspection(files rawFiles, workDir string) (string, error) {
	if err := requireFile(files.Raw); err == nil {
		return files.Raw, nil
	}

	raw := filepath.Join(workDir, strings.TrimSuffix(filepath.Base(files.Compressed), ".xz"))
	out, err := os.Create(raw)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(i.Context, "xz", "-dc", files.Compressed)
	cmd.Stdout = out
	cmd.Stderr = i.Stderr
	runErr := cmd.Run()
	closeErr := out.Close()
	if runErr != nil {
		return "", fmt.Errorf("decompressing %s for raw inspection failed: %w", files.Compressed, runErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := requireFile(raw); err != nil {
		return "", err
	}
	return raw, nil
}

func (i Inspector) inspectISO(resolved plan.Plan) (string, error) {
	iso := filepath.Join(resolved.ArtifactDir, resolved.ArtifactStem+".iso")
	if err := requireFile(iso); err != nil {
		return "", err
	}
	return iso, nil
}

func (i Inspector) OCI(resolved plan.Plan, imageOverride string) error {
	i = i.withDefaults()
	if i.Stdout == nil {
		i.Stdout = io.Discard
	}

	image := resolved.Image
	if imageOverride != "" {
		image = imageOverride
	}

	for _, absent := range resolved.Inspection.OCI.Absent {
		if err := i.runContainerShell(image, "[ ! -e "+shellQuote(absent.Path)+" ]"); err != nil {
			return fmt.Errorf("OCI path %s should be absent: %w", absent.Path, err)
		}
		fmt.Fprintf(i.Stdout, "OCI absent %s: OK\n", absent.Path)
	}
	for _, command := range resolved.Inspection.OCI.Commands {
		if err := i.runContainerShell(image, "command -v -- "+shellQuote(command.Name)+" >/dev/null"); err != nil {
			return fmt.Errorf("OCI command %s not found: %w", command.Name, err)
		}
		fmt.Fprintf(i.Stdout, "OCI command %s: OK\n", command.Name)
	}
	for _, file := range resolved.Inspection.OCI.Files {
		data, err := i.readContainerFile(image, file.Path)
		if err != nil {
			return err
		}
		if err := validateFileExpectation(data, file, "OCI file "); err != nil {
			return err
		}
		fmt.Fprintf(i.Stdout, "OCI file %s: OK\n", file.Path)
	}

	fmt.Fprintf(i.Stdout, "OCI inspection passed for %s\n", resolved.Target)
	return nil
}

func (i Inspector) withDefaults() Inspector {
	if i.Context == nil {
		i.Context = context.Background()
	}
	if i.Stdout == nil {
		i.Stdout = io.Discard
	}
	if i.Stderr == nil {
		i.Stderr = io.Discard
	}
	if i.Runner == nil {
		i.Runner = ExecRunner{}
	}
	return i
}

func (i Inspector) verifyChecksums(dir string, expectedFiles []string) error {
	checksums, err := ParseSHA256SUMS(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		return err
	}

	for _, file := range expectedFiles {
		name := filepath.Base(file)
		want, ok := checksums[name]
		if !ok {
			return fmt.Errorf("SHA256SUMS does not contain expected file %s", name)
		}

		got, err := sha256File(file)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("checksum mismatch for %s: got %s, want %s", name, got, want)
		}
		fmt.Fprintf(i.Stdout, "%s: OK\n", name)
	}
	return nil
}

func (i Inspector) readContainerFile(image string, path string) ([]byte, error) {
	var stdout bytes.Buffer
	if err := i.runContainerShellWithOutput(image, "cat -- "+shellQuote(path), &stdout); err != nil {
		return nil, fmt.Errorf("OCI file %s was not readable: %w", path, err)
	}
	return stdout.Bytes(), nil
}

func (i Inspector) runContainerShell(image string, script string) error {
	return i.runContainerShellWithOutput(image, script, io.Discard)
}

func (i Inspector) runContainerShellWithOutput(image string, script string, stdout io.Writer) error {
	args := []string{
		"run",
		"--rm",
		"--entrypoint", "/bin/sh",
		image,
		"-c", script,
	}
	return i.Runner.Run(i.Context, "docker", args, stdout, i.Stderr)
}

func RawFiles(resolved plan.Plan) rawFiles {
	raw := filepath.Join(resolved.ArtifactDir, resolved.ArtifactStem+".raw")
	return rawFiles{
		Raw:        raw,
		Compressed: raw + ".xz",
	}
}

func ParseSHA256SUMS(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("missing checksum file %s: %w", path, err)
	}
	defer file.Close()

	result := map[string]string{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s:%d: expected '<sha256> <filename>'", path, lineNo)
		}
		hash := strings.ToLower(fields[0])
		if len(hash) != sha256.Size*2 {
			return nil, fmt.Errorf("%s:%d: invalid sha256 length for %s", path, lineNo, fields[1])
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return nil, fmt.Errorf("%s:%d: invalid sha256 for %s: %w", path, lineNo, fields[1], err)
		}
		result[fields[1]] = hash
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s does not contain any checksums", path)
	}

	return result, nil
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected a file", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s is empty", path)
	}
	return nil
}

func rawPatchExtractRequests(patches []plan.RawPatch) ([]rawpatch.ExtractRequest, error) {
	var requests []rawpatch.ExtractRequest
	for _, patch := range patches {
		switch patch.Type {
		case "copy-to-partition":
			requests = append(requests, rawpatch.ExtractRequest{
				PartitionLabel: patch.PartitionLabel,
				Path:           patch.Path,
			})
		case "yaml-json-patch":
			requests = append(requests, rawpatch.ExtractRequest{
				PartitionLabel: patch.PartitionLabel,
				Path:           patch.TargetPath,
			})
		default:
			return nil, fmt.Errorf("unsupported raw patch type %q", patch.Type)
		}
	}
	return requests, nil
}

func rawExtractRequests(resolved plan.Plan) ([]rawpatch.ExtractRequest, error) {
	requests, err := rawPatchExtractRequests(resolved.RawPatches)
	if err != nil {
		return nil, err
	}
	for _, partition := range resolved.Inspection.Raw.Partitions {
		for _, file := range partition.Files {
			requests = append(requests, rawpatch.ExtractRequest{
				PartitionLabel: partition.Label,
				Path:           file.Path,
			})
		}
	}
	return requests, nil
}

func validateRawPatches(workDir string, resolved plan.Plan) error {
	for _, patch := range resolved.RawPatches {
		switch patch.Type {
		case "copy-to-partition":
			source := filepath.Join(resolved.Paths.OverlaysDir, filepath.FromSlash(patch.Overlay), filepath.FromSlash(patch.Source))
			want, err := os.ReadFile(source)
			if err != nil {
				return err
			}
			gotPath := filepath.Join(workDir, patch.PartitionLabel, filepath.FromSlash(patch.Path))
			got, err := os.ReadFile(gotPath)
			if err != nil {
				return fmt.Errorf("%s:%s was not extracted: %w", patch.PartitionLabel, patch.Path, err)
			}
			if !bytes.Equal(got, want) {
				return fmt.Errorf("%s:%s does not match %s", patch.PartitionLabel, patch.Path, source)
			}
		case "yaml-json-patch":
			target := filepath.Join(workDir, patch.PartitionLabel, filepath.FromSlash(patch.TargetPath))
			data, err := os.ReadFile(target)
			if err != nil {
				return fmt.Errorf("%s:%s was not extracted: %w", patch.PartitionLabel, patch.TargetPath, err)
			}
			if err := rawpatch.ValidateStructuredPatchResult(data, patch.TargetPath, patch.Operations); err != nil {
				return fmt.Errorf("%s:%s validation failed: %w", patch.PartitionLabel, patch.TargetPath, err)
			}
		default:
			return fmt.Errorf("unsupported raw patch type %q", patch.Type)
		}
	}
	return nil
}

func validateRawExpectations(workDir string, resolved plan.Plan) error {
	for _, partition := range resolved.Inspection.Raw.Partitions {
		for _, file := range partition.Files {
			target := filepath.Join(workDir, partition.Label, filepath.FromSlash(file.Path))
			data, err := os.ReadFile(target)
			if err != nil {
				return fmt.Errorf("%s:%s was not extracted: %w", partition.Label, file.Path, err)
			}
			if err := validateFileExpectation(data, file, partition.Label+":"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFileExpectation(data []byte, file plan.FileInspection, prefix string) error {
	for _, contains := range file.Contains {
		if !bytes.Contains(data, []byte(contains)) {
			return fmt.Errorf("%s%s does not contain %q", prefix, file.Path, contains)
		}
	}
	if len(file.StructuredTests) > 0 {
		if err := rawpatch.ValidateStructuredPatchResult(data, file.Path, file.StructuredTests); err != nil {
			return fmt.Errorf("%s%s structured validation failed: %w", prefix, file.Path, err)
		}
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func sha256File(path string) (string, error) {
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
