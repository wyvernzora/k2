package plan_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/config"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/paths"
	"github.com/wyvernzora/k2/tools/internal/kairos/imagebuild/plan"
)

func TestBuildPlanGolden(t *testing.T) {
	planner, fixture := newFixturePlanner(t)

	tests := []struct {
		target string
		golden string
	}{
		{
			target: "ubuntu-24.04-amd64-qemu-k8s",
			golden: "ubuntu-24.04-amd64-qemu-k8s.golden.json",
		},
		{
			target: "ubuntu-24.04-arm64-qemu-k8s",
			golden: "ubuntu-24.04-arm64-qemu-k8s.golden.json",
		},
		{
			target: "ubuntu-24.04-arm64-rpi4cb-k8s",
			golden: "ubuntu-24.04-arm64-rpi4cb-k8s.golden.json",
		},
		{
			target: "ubuntu-24.04-amd64-qemu-storage",
			golden: "ubuntu-24.04-amd64-qemu-storage.golden.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got, err := planner.Build(tt.target)
			if err != nil {
				t.Fatal(err)
			}
			assertGoldenJSON(t, got, tt.golden, fixture)
		})
	}
}

func TestEnabledTargets(t *testing.T) {
	planner, _ := newFixturePlanner(t)

	got := planner.EnabledTargets()
	want := []string{
		"ubuntu-24.04-amd64-qemu-k8s",
		"ubuntu-24.04-amd64-qemu-storage",
		"ubuntu-24.04-arm64-qemu-k8s",
		"ubuntu-24.04-arm64-rpi4cb-k8s",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("enabled targets = %#v, want %#v", got, want)
	}
}

func TestImageTagAndArtifactStemMatchShellContract(t *testing.T) {
	planner, _ := newFixturePlanner(t)

	got, err := planner.Build("ubuntu-24.04-arm64-rpi4cb-k8s")
	if err != nil {
		t.Fatal(err)
	}

	wantImage := "ghcr.io/wyvernzora/k2-kairos:ubuntu-24.04-v4.1.0-arm64-rpi4cb-k8s-v1.36.0-k3s1-rev0"
	if got.Image != wantImage {
		t.Fatalf("image = %q, want %q", got.Image, wantImage)
	}
	wantStem := "ubuntu-24.04-v4.1.0-arm64-rpi4cb-k8s-v1.36.0-k3s1-rev0"
	if got.ArtifactStem != wantStem {
		t.Fatalf("artifact stem = %q, want %q", got.ArtifactStem, wantStem)
	}
}

func TestImageTagOmitsKubernetesSegmentsForStorageTarget(t *testing.T) {
	planner, _ := newFixturePlanner(t)

	got, err := planner.Build("ubuntu-24.04-amd64-qemu-storage")
	if err != nil {
		t.Fatal(err)
	}

	if got.KubernetesDistro != "" {
		t.Fatalf("kubernetesDistro = %q, want empty", got.KubernetesDistro)
	}
	wantImage := "ghcr.io/wyvernzora/k2-kairos:ubuntu-24.04-v4.1.0-amd64-qemu-storage-rev0"
	if got.Image != wantImage {
		t.Fatalf("image = %q, want %q", got.Image, wantImage)
	}
	wantStem := "ubuntu-24.04-v4.1.0-amd64-qemu-storage-rev0"
	if got.ArtifactStem != wantStem {
		t.Fatalf("artifact stem = %q, want %q", got.ArtifactStem, wantStem)
	}
	if strings.Contains(got.Image, "--") {
		t.Fatalf("image %q contains empty tag segment", got.Image)
	}
}

func TestRawPatchRejectsUnsupportedPatchTarget(t *testing.T) {
	planner, _ := newFixturePlanner(t)
	unsupported := filepath.Join(planner.Paths.OverlaysDir, "hardware", "rpi4cb", "raw", "COS_GRUB", "extraconfig.txt.patch")
	mustWrite(t, unsupported, "- op: test\n  path: /value\n  value: nope\n")

	_, err := planner.Build("ubuntu-24.04-arm64-rpi4cb-k8s")
	if err == nil {
		t.Fatal("expected unsupported .txt.patch error")
	}
	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Fatalf("error = %v", err)
	}
}

func TestTargetsRejectUnknownFields(t *testing.T) {
	root := t.TempDir()
	targetsPath := filepath.Join(root, "targets.yaml")
	mustWrite(t, targetsPath, strings.TrimSpace(`
targets:
  ubuntu-24.04-amd64-qemu-k8s:
    enabled: true
    flavor: ubuntu-24.04
    flavorRelease: "24.04"
    role: k8s
    arch: amd64
    hardware: qemu
    kairosModel: generic
    artifacts:
      - raw
    overlays:
      - base
      - hardware/qemu
      - role/k8s
    artifactOptions:
      raw:
        diskSize: 8192
        diskStateSize: 4096

  ubuntu-24.04-arm64-qemu-k8s:
    enabled: true
    flavor: ubuntu-24.04
    role: k8s
    arch: arm64
    hardware: qemu
    kairosModel: generic
    artifacts:
      - raw
    overlays:
      - base
      - hardware/qemu
      - role/k8s
    artifactOptions:
      raw:
        diskSize: 8192
        diskStateSize: 4096

  ubuntu-24.04-arm64-rpi4cb-k8s:
    enabled: true
    flavor: ubuntu-24.04
    role: k8s
    arch: arm64
    hardware: rpi4cb
    kairosModel: rpi4
    artifacts:
      - raw
    overlays:
      - base
      - hardware/rpi4cb
`)+"\n")

	_, err := config.LoadTargets(targetsPath)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), `unknown target field "flavorRelease"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestInspectionRejectsConflictingStructuredTests(t *testing.T) {
	planner, _ := newFixturePlanner(t)
	mustWrite(t, filepath.Join(planner.Paths.OverlaysDir, "extra", "overlay.yaml"), strings.TrimSpace(`
inspect:
  raw:
    partitions:
      COS_OEM:
        files:
          - path: 01_reset.yaml
            structuredTests:
              - op: test
                path: /stages/rootfs.before/0/layout/add_partitions/1/size
                value: 250
`)+"\n")

	_, err := planner.Build("ubuntu-24.04-arm64-rpi4cb-k8s-extra")
	if err == nil {
		t.Fatal("expected conflicting inspection error")
	}
	if !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("error = %v", err)
	}
}

func TestInspectionRejectsAbsentFileConflict(t *testing.T) {
	planner, _ := newFixturePlanner(t)
	mustWrite(t, filepath.Join(planner.Paths.OverlaysDir, "extra", "overlay.yaml"), strings.TrimSpace(`
inspect:
  oci:
    absent:
      - /system/oem/05-persistent-storage.yaml
`)+"\n")

	_, err := planner.Build("ubuntu-24.04-arm64-rpi4cb-k8s-extra")
	if err == nil {
		t.Fatal("expected absent/file conflict")
	}
	if !strings.Contains(err.Error(), "also has file expectations") {
		t.Fatalf("error = %v", err)
	}
}

func newFixturePlanner(t *testing.T) (planner plan.Planner, root string) {
	t.Helper()

	root = t.TempDir()
	buildRoot := filepath.Join(root, "kairos")
	kairosRoot := filepath.Join(root, "kairos")
	mustMkdir(t, buildRoot)
	mustWrite(t, filepath.Join(buildRoot, "Dockerfile"), "FROM scratch\n")

	mustWrite(t, filepath.Join(kairosRoot, "versions.env"), strings.TrimSpace(`
KAIROS_VERSION=v4.1.0
KAIROS_INIT_VERSION=v0.13.0
AURORABOOT_VERSION=v0.19.4
BASE_IMAGE=ubuntu:24.04
K3S_VERSION=v1.36.0+k3s1
K2_IMAGE_REVISION=rev0
REGISTRY_IMAGE=ghcr.io/wyvernzora/k2-kairos
`)+"\n")
	mustWrite(t, filepath.Join(kairosRoot, "targets.yaml"), strings.TrimSpace(`
targets:
  ubuntu-24.04-amd64-qemu-k8s:
    enabled: true
    flavor: ubuntu-24.04
    role: k8s
    arch: amd64
    hardware: qemu
    kairosModel: generic
    artifacts:
      - raw
    overlays:
      - base
      - hardware/qemu
      - role/k8s
    artifactOptions:
      raw:
        diskSize: 8192
        diskStateSize: 4096

  ubuntu-24.04-arm64-qemu-k8s:
    enabled: true
    flavor: ubuntu-24.04
    role: k8s
    arch: arm64
    hardware: qemu
    kairosModel: generic
    artifacts:
      - raw
    overlays:
      - base
      - hardware/qemu
      - role/k8s
    artifactOptions:
      raw:
        diskSize: 8192
        diskStateSize: 4096

  ubuntu-24.04-arm64-rpi4cb-k8s:
    enabled: true
    flavor: ubuntu-24.04
    role: k8s
    arch: arm64
    hardware: rpi4cb
    kairosModel: rpi4
    artifacts:
      - raw
    overlays:
      - base
      - hardware/rpi4cb
      - role/k8s
    artifactOptions:
      raw:
        diskStateSize: 8192

  ubuntu-24.04-amd64-qemu-storage:
    enabled: true
    flavor: ubuntu-24.04
    role: storage
    arch: amd64
    hardware: qemu
    kairosModel: generic
    artifacts:
      - raw
    overlays:
      - base
      - hardware/qemu
      - role/storage
    artifactOptions:
      raw:
        diskSize: 16384
        diskStateSize: 6144

  ubuntu-24.04-arm64-rpi4cb-k8s-extra:
    enabled: false
    inherits: ubuntu-24.04-arm64-rpi4cb-k8s
    overlays:
      - extra
`)+"\n")

	mustWrite(t, filepath.Join(buildRoot, "overlays", "base", "README.md"), "# Base Overlay\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "base", "overlay.yaml"), strings.TrimSpace(`
inspect:
  oci:
    files:
      - path: /system/oem/01-k2-rescue-deactivate.yaml
        contains:
          - K2 rescue user
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "hardware", "qemu", "README.md"), "# qemu Hardware Overlay\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "hardware", "qemu", "overlay.yaml"), strings.TrimSpace(`
build:
  aptPackages:
    - qemu-guest-agent
inspect:
  oci:
    files:
      - path: /system/oem/07-qemu-guest-agent.yaml
        contains:
          - QEMU guest agent
    absent:
      - /system/oem/06-qemu-serial-console.yaml
    commands:
      - qemu-ga
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "hardware", "qemu", "oci", "system", "oem", "07-qemu-guest-agent.yaml"), "#cloud-config\nname: QEMU guest agent\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "hardware", "rpi4cb", "raw", "COS_GRUB", "extraconfig.txt"), "dtparam=pciex1\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "hardware", "rpi4cb", "raw", "COS_OEM", "01_reset.yaml.patch"), strings.TrimSpace(`
- op: test
  path: /stages/rootfs.before/0/layout/add_partitions/1/fsLabel
  value: COS_PERSISTENT
- op: add
  path: /stages/rootfs.before/0/layout/add_partitions/1/size
  value: 500
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "hardware", "rpi4cb", "overlay.yaml"), strings.TrimSpace(`
inspect:
  oci:
    absent:
      - /system/oem/20-rpi4cb-nvme-data.yaml
  raw:
    partitions:
      COS_GRUB:
        files:
          - path: extraconfig.txt
            contains:
              - dtparam=pciex1
      COS_OEM:
        files:
          - path: 01_reset.yaml
            structuredTests:
              - op: test
                path: /stages/rootfs.before/0/layout/add_partitions/1/size
                value: 500
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "role", "k8s", "oci", "usr", "share", "k2", "node-provision", "k3s", "README.md"), strings.TrimSpace(`
# K2 K3s Node Provisioning Overlay

The overlay installs invariant K2 K3s server configuration as inert files in /usr/share/k2.
Active cluster-specific K3s configuration is written at provision time.
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "role", "k8s", "oci", "usr", "share", "k2", "node-provision", "k3s", "10-k2-invariant.yaml"), strings.TrimSpace(`
flannel-backend: none
disable-kube-proxy: true
disable-helm-controller: true
secrets-encryption: true
secrets-encryption-provider: secretbox
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "role", "k8s", "oci", "system", "oem", "05-persistent-storage.yaml"), "#cloud-config\nname: K2 persistent storage\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "role", "k8s", "overlay.yaml"), strings.TrimSpace(`
build:
  aptPackages:
    - parted
    - util-linux
  dracutInstallItems:
    - /usr/sbin/k2-node-agent
    - /usr/sbin/parted
inspect:
  oci:
    files:
      - path: /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml
        contains:
          - "flannel-backend: none"
          - "disable-kube-proxy: true"
          - "secrets-encryption-provider: secretbox"
      - path: /usr/share/k2/node-provision/k3s/README.md
        contains:
          - K2 K3s Node Provisioning Overlay
          - Active cluster-specific K3s configuration is written at provision time
      - path: /system/oem/05-persistent-storage.yaml
        contains:
          - K2 persistent storage
    absent:
      - /system/oem/30-k2-k3s-provider.yaml
      - /etc/rancher/k3s/config.yaml
      - /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml
      - /etc/rancher/k3s/config.yaml.d/20-k2-intent.yaml
    commands:
      - k2-node-agent
      - parted
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "role", "storage", "README.md"), "# Storage Role Overlay\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "role", "storage", "oci", "system", "oem", "10-storage-services.yaml"), "#cloud-config\nname: K2 storage services\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "role", "storage", "overlay.yaml"), strings.TrimSpace(`
build:
  aptPackages:
    - targetcli-fb
    - zfsutils-linux
  postInstall:
    - zfs-hostid
inspect:
  oci:
    files:
      - path: /system/oem/10-storage-services.yaml
        contains:
          - K2 storage services
    absent:
      - /etc/rancher
      - /system/oem/05-persistent-storage.yaml
    commands:
      - zfs
      - targetcli
`)+"\n")
	mustWrite(t, filepath.Join(buildRoot, "overlays", "extra", ".gitkeep"), "")

	discovered, err := paths.Discover(buildRoot, paths.Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	targets, err := config.LoadTargets(discovered.TargetsFile)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := config.LoadVersions(discovered.VersionsFile)
	if err != nil {
		t.Fatal(err)
	}

	return plan.New(targets, versions, discovered), filepath.ToSlash(root)
}

func assertGoldenJSON(t *testing.T, value any, golden string, fixture string) {
	t.Helper()

	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.ReplaceAll(filepath.ToSlash(string(encoded)), fixture, "$FIXTURE") + "\n"
	wantBytes, err := os.ReadFile(filepath.Join("testdata", golden))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(wantBytes) {
		t.Fatalf("golden mismatch for %s\ngot:\n%s\nwant:\n%s", golden, got, string(wantBytes))
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
