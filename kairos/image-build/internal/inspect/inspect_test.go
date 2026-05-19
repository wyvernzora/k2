package inspect

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wyvernzora/k2/kairos/image-build/internal/plan"
)

func TestParseSHA256SUMS(t *testing.T) {
	dir := t.TempDir()
	sums := filepath.Join(dir, "SHA256SUMS")
	hash := strings.Repeat("a", sha256.Size*2)
	if err := os.WriteFile(sums, []byte(hash+"  artifact.raw\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ParseSHA256SUMS(sums)
	if err != nil {
		t.Fatal(err)
	}
	if got["artifact.raw"] != hash {
		t.Fatalf("hash = %q, want %q", got["artifact.raw"], hash)
	}
}

func TestParseSHA256SUMSRejectsInvalidHash(t *testing.T) {
	dir := t.TempDir()
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(sums, []byte("not-a-hash  artifact.raw\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ParseSHA256SUMS(sums); err == nil {
		t.Fatal("expected invalid hash error")
	}
}

func TestRawFiles(t *testing.T) {
	resolved := plan.Plan{
		ArtifactDir:  "/tmp/artifacts/target",
		ArtifactStem: "image-stem",
	}

	got := RawFiles(resolved)
	if got.Raw != "/tmp/artifacts/target/image-stem.raw" {
		t.Fatalf("raw = %q", got.Raw)
	}
	if got.Compressed != "/tmp/artifacts/target/image-stem.raw.xz" {
		t.Fatalf("compressed = %q", got.Compressed)
	}
}

func TestValidateRawPatches(t *testing.T) {
	root := t.TempDir()
	overlaysDir := filepath.Join(root, "overlays")
	workDir := filepath.Join(root, "work")
	writeFile(t, filepath.Join(overlaysDir, "hardware", "rpi4cb", "raw", "COS_GRUB", "extraconfig.txt"), []byte("dtparam=pciex1\n"))
	writeFile(t, filepath.Join(workDir, "COS_GRUB", "extraconfig.txt"), []byte("dtparam=pciex1\n"))
	writeFile(t, filepath.Join(workDir, "COS_OEM", "01_reset.yaml"), []byte(`
stages:
  rootfs.before:
    - layout:
        add_partitions:
          - fsLabel: COS_STATE
            size: 8192
          - fsLabel: COS_PERSISTENT
            size: 500
`))

	resolved := plan.Plan{
		Paths: plan.PlanPaths{OverlaysDir: overlaysDir},
		RawPatches: []plan.RawPatch{
			{
				Type:           "copy-to-partition",
				Overlay:        "hardware/rpi4cb",
				Source:         "raw/COS_GRUB/extraconfig.txt",
				PartitionLabel: "COS_GRUB",
				Path:           "extraconfig.txt",
			},
			{
				Type:           "yaml-json-patch",
				PartitionLabel: "COS_OEM",
				TargetPath:     "01_reset.yaml",
				Operations: []plan.JSONPatchOperation{
					{Op: "test", Path: "/stages/rootfs.before/0/layout/add_partitions/1/fsLabel", Value: "COS_PERSISTENT"},
					{Op: "replace", Path: "/stages/rootfs.before/0/layout/add_partitions/1/size", Value: 500},
				},
			},
		},
	}

	if err := validateRawPatches(workDir, resolved); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRawExpectations(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "COS_GRUB", "extraconfig.txt"), []byte("dtparam=pciex1\n"))
	writeFile(t, filepath.Join(workDir, "COS_OEM", "01_reset.yaml"), []byte(`
stages:
  rootfs.before:
    - layout:
        add_partitions:
          - fsLabel: COS_STATE
            size: 8192
          - fsLabel: COS_PERSISTENT
            size: 500
`))

	err := validateRawExpectations(workDir, plan.Plan{
		Inspection: plan.Inspection{
			Raw: plan.RawInspection{
				Partitions: []plan.RawPartitionInspection{
					{
						Label: "COS_GRUB",
						Files: []plan.FileInspection{
							{Path: "extraconfig.txt", Contains: []string{"dtparam=pciex1"}},
						},
					},
					{
						Label: "COS_OEM",
						Files: []plan.FileInspection{
							{
								Path: "01_reset.yaml",
								StructuredTests: []plan.JSONPatchOperation{
									{Op: "test", Path: "/stages/rootfs.before/0/layout/add_partitions/1/size", Value: 500},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOCIInspection(t *testing.T) {
	runner := &fakeInspectRunner{
		files: map[string]string{
			"/system/oem/test.yaml": "name: test\n",
		},
	}

	var stdout bytes.Buffer
	err := (Inspector{
		Stdout: &stdout,
		Stderr: io.Discard,
		Runner: runner,
	}).OCI(plan.Plan{
		Target: "target",
		Image:  "image:test",
		Inspection: plan.Inspection{
			OCI: plan.OCIInspection{
				Absent: []plan.PathInspection{{Path: "/missing"}},
				Commands: []plan.CommandInspection{
					{Name: "parted"},
				},
				Files: []plan.FileInspection{
					{
						Path:     "/system/oem/test.yaml",
						Contains: []string{"name:"},
						StructuredTests: []plan.JSONPatchOperation{
							{Op: "test", Path: "/name", Value: "test"},
						},
					},
				},
			},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "OCI inspection passed for target") {
		t.Fatalf("stdout missing pass message: %s", stdout.String())
	}
}

func TestOCIInspectionFileFailure(t *testing.T) {
	runner := &fakeInspectRunner{
		files: map[string]string{
			"/system/oem/test.yaml": "name: test\n",
		},
	}

	err := (Inspector{Runner: runner}).OCI(plan.Plan{
		Target: "target",
		Image:  "image:test",
		Inspection: plan.Inspection{
			OCI: plan.OCIInspection{
				Files: []plan.FileInspection{
					{Path: "/system/oem/test.yaml", Contains: []string{"missing"}},
				},
			},
		},
	}, "")
	if err == nil {
		t.Fatal("expected contains failure")
	}
}

func TestInspectISO(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "image.iso")
	writeFile(t, iso, []byte("iso"))
	writeChecksums(t, dir, iso)

	var stdout bytes.Buffer
	err := (Inspector{Stdout: &stdout}).Artifact(plan.Plan{
		Target:       "iso-target",
		Artifacts:    []string{"iso"},
		ArtifactDir:  dir,
		ArtifactStem: "image",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "image.iso: OK") {
		t.Fatalf("stdout did not include checksum OK: %s", stdout.String())
	}
}

func TestInspectISOMissing(t *testing.T) {
	dir := t.TempDir()

	err := (Inspector{}).Artifact(plan.Plan{
		Target:       "iso-target",
		Artifacts:    []string{"iso"},
		ArtifactDir:  dir,
		ArtifactStem: "image",
	})
	if err == nil {
		t.Fatal("expected missing ISO error")
	}
}

func TestChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "image.iso")
	writeFile(t, iso, []byte("iso"))
	writeFile(t, filepath.Join(dir, "SHA256SUMS"), []byte(strings.Repeat("0", sha256.Size*2)+"  image.iso\n"))

	err := (Inspector{}).Artifact(plan.Plan{
		Target:       "iso-target",
		Artifacts:    []string{"iso"},
		ArtifactDir:  dir,
		ArtifactStem: "image",
	})
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func writeChecksums(t *testing.T, dir string, files ...string) {
	t.Helper()

	var lines []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		lines = append(lines, hex.EncodeToString(sum[:])+"  "+filepath.Base(file))
	}
	writeFile(t, filepath.Join(dir, "SHA256SUMS"), []byte(strings.Join(lines, "\n")+"\n"))
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

type fakeInspectRunner struct {
	files map[string]string
}

func (r *fakeInspectRunner) Run(name string, args []string, stdout io.Writer, stderr io.Writer) error {
	if name != "docker" {
		return errors.New("unexpected command " + name)
	}
	if len(args) == 0 {
		return errors.New("missing docker args")
	}
	script := args[len(args)-1]
	switch {
	case strings.HasPrefix(script, "cat -- "):
		path := strings.Trim(strings.TrimPrefix(script, "cat -- "), "'")
		data, ok := r.files[path]
		if !ok {
			return errors.New("missing file " + path)
		}
		_, err := io.WriteString(stdout, data)
		return err
	default:
		return nil
	}
}
