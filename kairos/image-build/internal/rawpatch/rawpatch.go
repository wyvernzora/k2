package rawpatch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/wyvernzora/k2/kairos/image-build/internal/plan"
)

type Patcher struct {
	Stdout io.Writer
	Stderr io.Writer
	Runner CommandRunner
}

type CommandRunner interface {
	Run(name string, args []string, stdout io.Writer, stderr io.Writer) error
}

type ExecRunner struct{}

type ExtractRequest struct {
	PartitionLabel string
	Path           string
}

type partitionWork struct {
	Label        string
	ExtractPaths []string
}

func (ExecRunner) Run(name string, args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (p Patcher) Patch(rawFile string, resolved plan.Plan) error {
	if len(resolved.RawPatches) == 0 {
		return nil
	}
	if p.Stdout == nil {
		p.Stdout = io.Discard
	}
	if p.Stderr == nil {
		p.Stderr = io.Discard
	}
	if p.Runner == nil {
		p.Runner = ExecRunner{}
	}

	workDir, err := os.MkdirTemp("", "k2-kairos-rawpatch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	partitionDir := filepath.Join(workDir, "partition")
	if err := os.MkdirAll(partitionDir, 0o755); err != nil {
		return err
	}

	work, err := preparePartitionWork(resolved.RawPatches)
	if err != nil {
		return err
	}

	for _, partition := range work {
		if len(partition.ExtractPaths) == 0 {
			continue
		}
		if err := writePathList(workDir, partition.ExtractPaths); err != nil {
			return err
		}
		if err := p.runPartitionHelper(rawFile, resolved.Versions.AuroraBootVersion, partition.Label, workDir, "extract"); err != nil {
			return err
		}
	}

	for _, patch := range resolved.RawPatches {
		if err := p.applyPatchOperation(partitionDir, resolved, patch); err != nil {
			return err
		}
	}

	beforeApply, err := os.Stat(rawFile)
	if err != nil {
		return err
	}
	beforeApplyHash := ""
	if runtime.GOOS == "darwin" {
		beforeApplyHash, err = sha256File(rawFile)
		if err != nil {
			return err
		}
	}
	applied := dirHasContent(partitionDir)
	if applied {
		if err := p.runPartitionHelper(rawFile, resolved.Versions.AuroraBootVersion, "", workDir, "apply-all"); err != nil {
			return err
		}
	}
	if applied && runtime.GOOS == "darwin" {
		fmt.Fprintln(p.Stdout, "Waiting for Docker Desktop raw image writes to settle")
		if err := waitForHostVisibleRawChange(rawFile, beforeApply.ModTime(), beforeApplyHash, 5*time.Minute); err != nil {
			return err
		}
	}
	return nil
}

func ExtractFiles(rawFile string, auroraBootVersion string, requests []ExtractRequest, destination string) error {
	if len(requests) == 0 {
		return nil
	}
	workDir, err := os.MkdirTemp("", "k2-kairos-rawextract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	byPartition := map[string][]string{}
	for _, request := range requests {
		if err := validatePartitionPath(request.Path); err != nil {
			return err
		}
		byPartition[request.PartitionLabel] = append(byPartition[request.PartitionLabel], request.Path)
	}

	partitionDir := filepath.Join(workDir, "partition")
	if err := os.MkdirAll(partitionDir, 0o755); err != nil {
		return err
	}

	runner := ExecRunner{}
	for _, label := range sortedKeys(byPartition) {
		paths := dedupeStrings(byPartition[label])
		if err := writePathList(workDir, paths); err != nil {
			return err
		}
		if err := runPartitionHelper(runner, rawFile, auroraBootVersion, label, workDir, "extract", io.Discard, io.Discard); err != nil {
			return err
		}
		src := filepath.Join(partitionDir, label)
		if dirHasContent(src) {
			if err := copyTree(src, filepath.Join(destination, label)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p Patcher) applyPatchOperation(partitionDir string, resolved plan.Plan, patch plan.RawPatch) error {
	switch patch.Type {
	case "copy-to-partition":
		source := filepath.Join(resolved.Paths.OverlaysDir, filepath.FromSlash(patch.Overlay), filepath.FromSlash(patch.Source))
		destination := filepath.Join(partitionDir, patch.PartitionLabel, filepath.FromSlash(patch.Path))
		if err := validatePartitionPath(patch.Path); err != nil {
			return err
		}
		fmt.Fprintf(p.Stdout, "Copying %s to %s:%s\n", source, patch.PartitionLabel, patch.Path)
		return copyFile(source, destination)
	case "yaml-json-patch":
		if err := validatePartitionPath(patch.TargetPath); err != nil {
			return err
		}
		target := filepath.Join(partitionDir, patch.PartitionLabel, filepath.FromSlash(patch.TargetPath))
		data, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("%s:%s patch target is missing: %w", patch.PartitionLabel, patch.TargetPath, err)
		}
		fmt.Fprintf(p.Stdout, "Applying %s to %s:%s\n", patch.Source, patch.PartitionLabel, patch.TargetPath)
		patched, err := ApplyStructuredPatch(data, patch.TargetPath, patch.Operations)
		if err != nil {
			return fmt.Errorf("apply %s: %w", patch.Source, err)
		}
		return os.WriteFile(target, patched, 0o644)
	default:
		return fmt.Errorf("unsupported raw patch type %q", patch.Type)
	}
}

func preparePartitionWork(patches []plan.RawPatch) ([]partitionWork, error) {
	byPartition := map[string]map[string]bool{}
	for _, patch := range patches {
		if patch.PartitionLabel == "" {
			return nil, fmt.Errorf("raw patch %s is missing partition label", patch.Source)
		}
		if byPartition[patch.PartitionLabel] == nil {
			byPartition[patch.PartitionLabel] = map[string]bool{}
		}
		if patch.Type == "yaml-json-patch" {
			if err := validatePartitionPath(patch.TargetPath); err != nil {
				return nil, err
			}
			byPartition[patch.PartitionLabel][patch.TargetPath] = true
		}
	}

	var result []partitionWork
	for _, label := range sortedKeys(byPartition) {
		var paths []string
		for path := range byPartition[label] {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		result = append(result, partitionWork{
			Label:        label,
			ExtractPaths: paths,
		})
	}
	return result, nil
}

func (p Patcher) runPartitionHelper(rawFile string, auroraBootVersion string, label string, workDir string, mode string) error {
	return runPartitionHelper(p.Runner, rawFile, auroraBootVersion, label, workDir, mode, p.Stdout, p.Stderr)
}

func runPartitionHelper(runner CommandRunner, rawFile string, auroraBootVersion string, label string, workDir string, mode string, stdout io.Writer, stderr io.Writer) error {
	rawDir := filepath.Dir(rawFile)
	rawName := filepath.Base(rawFile)
	args := []string{
		"run",
		"--rm",
		"--privileged",
		"-e", "PARTITION_LABEL=" + label,
		"-e", "PATCH_MODE=" + mode,
		"-e", "RAW_FILE_NAME=" + rawName,
		"-v", rawDir + ":/image-dir",
		"-v", workDir + ":/work",
		"--entrypoint", "/bin/sh",
		"quay.io/kairos/auroraboot:" + auroraBootVersion,
		"-c", partitionHelperScript,
	}
	if err := runner.Run("docker", args, stdout, stderr); err != nil {
		return fmt.Errorf("raw partition %s %s failed: %w", label, mode, err)
	}
	return nil
}

func writePathList(workDir string, paths []string) error {
	var builder strings.Builder
	for _, path := range paths {
		if err := validatePartitionPath(path); err != nil {
			return err
		}
		builder.WriteString(path)
		builder.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(workDir, "paths.txt"), []byte(builder.String()), 0o644)
}

func validatePartitionPath(path string) error {
	if path == "" {
		return fmt.Errorf("partition path must not be empty")
	}
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return fmt.Errorf("partition path %q must be relative", path)
	}
	if strings.Contains(path, "\n") {
		return fmt.Errorf("partition path %q must not contain newlines", path)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("partition path %q must not escape the partition root", path)
	}
	return nil
}

func copyFile(source string, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}

func copyTree(source string, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeType != 0 {
			return nil
		}
		return copyFile(path, target)
	})
}

func dirHasContent(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func sortedKeys[T any](items map[string]T) []string {
	var keys []string
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func dedupeStrings(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sort.Strings(out)
	return out
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

func waitForHostVisibleRawChange(path string, previousModTime time.Time, previousHash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastSeenHash string
	var lastChange time.Time
	changed := false

	for {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		currentHash, err := sha256File(path)
		if err != nil {
			return err
		}
		if currentHash != previousHash {
			if !changed || currentHash != lastSeenHash {
				changed = true
				lastSeenHash = currentHash
				lastChange = time.Now()
			}
			if time.Since(lastChange) >= 30*time.Second {
				return nil
			}
		}

		if time.Now().After(deadline) {
			if !info.ModTime().Equal(previousModTime) {
				return fmt.Errorf("raw image %s changed timestamp after partition patching, but host-visible bytes did not change", path)
			}
			return fmt.Errorf("raw image %s did not become host-visible after partition patching", path)
		}
		time.Sleep(5 * time.Second)
	}
}

const partitionHelperScript = `
set -eu

host_raw_file="/image-dir/${RAW_FILE_NAME}"
raw_file="${host_raw_file}"
mount_dir=""
loopdev=""

cleanup() {
    if [ -n "${mount_dir}" ] && mountpoint -q "${mount_dir}" 2>/dev/null; then
        umount "${mount_dir}" || true
    fi
    if [ -n "${loopdev}" ]; then
        losetup -d "${loopdev}" || true
    fi
}
trap cleanup EXIT

mount_partition() {
    parts="$(parted -m "${raw_file}" unit B print | grep "^[0-9][0-9]*:")"
    while IFS=: read -r number start end size rest; do
        start="${start%B}"
        size="${size%B}"
        candidate="$(losetup --find --show --offset "${start}" --sizelimit "${size}" "${raw_file}")"
        label="$(blkid -p -s LABEL -o value "${candidate}" 2>/dev/null || true)"
        if [ "${label}" = "${PARTITION_LABEL}" ]; then
            loopdev="${candidate}"
            mount_dir="/mnt/raw-partition"
            mkdir -p "${mount_dir}"
            if [ "${PATCH_MODE}" = "extract" ]; then
                mount -o ro "${loopdev}" "${mount_dir}"
            else
                mount "${loopdev}" "${mount_dir}"
            fi
            return 0
        fi
        losetup -d "${candidate}" || true
    done <<EOF
${parts}
EOF

    echo "Unable to find partition label ${PARTITION_LABEL} in ${raw_file}" >&2
    exit 1
}

case "${PATCH_MODE}" in
    extract)
        mount_partition
        mkdir -p "/work/partition/${PARTITION_LABEL}"
        while IFS= read -r rel || [ -n "${rel}" ]; do
            [ -n "${rel}" ] || continue
            if [ ! -e "${mount_dir}/${rel}" ]; then
                echo "Missing ${PARTITION_LABEL}:${rel}" >&2
                exit 1
            fi
            mkdir -p "/work/partition/${PARTITION_LABEL}/$(dirname "${rel}")"
            cp -a "${mount_dir}/${rel}" "/work/partition/${PARTITION_LABEL}/${rel}"
        done < /work/paths.txt
        ;;
    apply-all)
        local_raw_file="/tmp/image.raw"
        cp "${host_raw_file}" "${local_raw_file}"
        raw_file="${local_raw_file}"

        for partition_dir in /work/partition/*; do
            [ -d "${partition_dir}" ] || continue
            PARTITION_LABEL="$(basename "${partition_dir}")"
            mount_partition
            cp -a "${partition_dir}/." "${mount_dir}/"
            sync
            cleanup
            mount_dir=""
            loopdev=""
        done

        cp "${local_raw_file}" "${host_raw_file}.patched"
        mv "${host_raw_file}.patched" "${host_raw_file}"
        sync
        ;;
    apply)
        mount_partition
        if [ -d "/work/partition/${PARTITION_LABEL}" ]; then
            cp -a "/work/partition/${PARTITION_LABEL}/." "${mount_dir}/"
            sync
        fi
        ;;
    *)
        echo "Unsupported PATCH_MODE ${PATCH_MODE}" >&2
        exit 1
        ;;
esac
`
