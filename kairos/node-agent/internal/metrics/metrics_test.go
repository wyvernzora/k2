package metrics

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectZFSPools(t *testing.T) {
	c := testCollector(fakeRunner{
		outputs: map[string]string{
			"zpool list -Hp -o name,size,alloc,frag,health": "tank\t1000\t255\t12\tONLINE\nbad line",
		},
	})

	got := c.collectZFSPools()
	if got.success {
		t.Fatal("success = true, want false for malformed line")
	}
	assertSample(t, got.samples, c.zfsPoolHealth, []string{"tank"}, 1)
	assertSample(t, got.samples, c.zfsPoolSize, []string{"tank"}, 1000)
	assertSample(t, got.samples, c.zfsPoolAlloc, []string{"tank"}, 255)
	assertSample(t, got.samples, c.zfsPoolFrag, []string{"tank"}, 0.12)
	assertSample(t, got.samples, c.zfsPoolCap, []string{"tank"}, 0.255)
}

func TestCollectZFSKeyStatus(t *testing.T) {
	c := testCollector(fakeRunner{
		outputs: map[string]string{
			"zpool list -Hp -o name": "tank",
			"zfs get -Hp -o name,value keystatus -r -t filesystem tank": strings.Join([]string{
				"tank\tavailable",
				"tank/plain\t-",
				"tank/locked\tunavailable",
				"malformed",
			}, "\n"),
		},
	})

	got := c.collectZFSKeyStatus()
	if got.success {
		t.Fatal("success = true, want false for malformed line")
	}
	assertSample(t, got.samples, c.zfsKeyStatus, []string{"tank"}, 1)
	assertSample(t, got.samples, c.zfsKeyStatus, []string{"tank/locked"}, 0)
	assertNoSample(t, got.samples, c.zfsKeyStatus, []string{"tank/plain"})
}

func TestCollectZFSVolumes(t *testing.T) {
	c := testCollector(fakeRunner{
		outputs: map[string]string{
			"zfs list -Hp -t volume -o name,volsize,used,usedbydataset,usedbysnapshots": "tank/vol1\t1073741824\t4096\t3072\t1024\nbroken",
		},
	})

	got := c.collectZFSVolumes()
	if got.success {
		t.Fatal("success = true, want false for malformed line")
	}
	assertSample(t, got.samples, c.zfsVolumeSize, []string{"tank/vol1"}, 1073741824)
	assertSample(t, got.samples, c.zfsVolumeUsed, []string{"tank/vol1"}, 4096)
	assertSample(t, got.samples, c.zfsVolumeDataset, []string{"tank/vol1"}, 3072)
	assertSample(t, got.samples, c.zfsVolumeSnapshot, []string{"tank/vol1"}, 1024)
	assertSample(t, got.samples, c.zfsVolumes, nil, 1)
}

func TestCollectLIO(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "iqn.2026-07.io.wyvernzora.k2:storage", "tpgt_1", "lun", "lun_0"))
	mustMkdir(t, filepath.Join(dir, "iqn.2026-07.io.wyvernzora.k2:storage", "tpgt_1", "acls", "iqn.client"))
	mustMkdir(t, filepath.Join(dir, "not-iqn", "tpgt_1", "lun", "lun_0"))
	saveConfig := filepath.Join(t.TempDir(), "saveconfig.json")
	if err := os.WriteFile(saveConfig, []byte(`{"targets":[{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := testCollector(fakeRunner{})
	c.cfg.ConfigFSRoot = dir
	c.cfg.SaveConfig = saveConfig

	got := c.collectLIO()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	assertSample(t, got.samples, c.lioTargets, nil, 1)
	assertSample(t, got.samples, c.lioLUNs, nil, 1)
	assertSample(t, got.samples, c.lioSessions, nil, 1)
	assertSample(t, got.samples, c.lioSaveInSync, nil, 1)
}

func TestCollectLIOUnparseableSaveConfigIsOutOfSync(t *testing.T) {
	dir := t.TempDir()
	saveConfig := filepath.Join(t.TempDir(), "saveconfig.json")
	if err := os.WriteFile(saveConfig, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := testCollector(fakeRunner{})
	c.cfg.ConfigFSRoot = dir
	c.cfg.SaveConfig = saveConfig

	got := c.collectLIO()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	assertSample(t, got.samples, c.lioSaveInSync, nil, 0)
}

// An absent saveconfig.json restores nothing at boot, so it must not read as a
// clean bill of health — least of all when configfs is unreadable too and the
// live target count is 0 for want of any evidence.
func TestCollectLIOMissingSaveConfigIsOutOfSync(t *testing.T) {
	c := testCollector(fakeRunner{})
	c.cfg.ConfigFSRoot = t.TempDir()
	c.cfg.SaveConfig = filepath.Join(t.TempDir(), "missing.json")

	got := c.collectLIO()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	assertSample(t, got.samples, c.lioSaveInSync, nil, 0)
}

func TestCollectSMART(t *testing.T) {
	c := testCollector(fakeRunner{
		outputs: map[string]string{
			"smartctl --scan -j": `{"devices":[{"name":"/dev/nvme0n1"},{"name":"/dev/sda"}]}`,
			"smartctl -aj /dev/nvme0n1": `{
				"temperature":{"current":37},
				"nvme_smart_health_information_log":{"percentage_used":2,"media_errors":3},
				"power_on_time":{"hours":456}
			}`,
		},
		outErrs: map[string]error{
			"smartctl -aj /dev/sda": errors.New("SMART unsupported"),
		},
	})

	got := c.collectSMART()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	assertSample(t, got.samples, c.smartTemp, []string{"/dev/nvme0n1"}, 37)
	assertSample(t, got.samples, c.smartPctUsed, []string{"/dev/nvme0n1"}, 2)
	assertSample(t, got.samples, c.smartMediaErrors, []string{"/dev/nvme0n1"}, 3)
	assertSample(t, got.samples, c.smartPowerHours, []string{"/dev/nvme0n1"}, 456)
	assertNoSample(t, got.samples, c.smartTemp, []string{"/dev/sda"})
}

func TestCollectSMARTScanFailure(t *testing.T) {
	c := testCollector(fakeRunner{
		outErrs: map[string]error{
			"smartctl --scan -j": errors.New("smartctl not found"),
		},
	})

	got := c.collectSMART()
	if got.success {
		t.Fatal("success = true, want false")
	}
	if len(got.samples) != 0 {
		t.Fatalf("samples = %d, want 0", len(got.samples))
	}
}

func TestCollectStorageHealth(t *testing.T) {
	status := filepath.Join(t.TempDir(), "status")
	if err := os.WriteFile(status, []byte("healthy: ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := testCollector(fakeRunner{})
	c.cfg.StatusFile = status

	got := c.collectStorageHealth()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	assertSample(t, got.samples, c.storageHealthy, nil, 1)
	if !hasSample(got.samples, c.storageLastRun, nil) {
		t.Fatal("missing storage last run sample")
	}
}

func TestCollectNodeBuild(t *testing.T) {
	metadataFile := filepath.Join(t.TempDir(), "metadata.yaml")
	metadata := strings.Join([]string{
		"target: ubuntu-26.04-amd64-qemu-k8s",
		" flavor : ubuntu-26.04 ",
		"kairosVersion: v4.1.2",
		"kubernetesDistro: k3s",
		"kubernetesVersion: v1.32.6+k3s1",
		"role: k8s",
		"arch: amd64",
		"hardware: qemu",
		"sourceCommit: ea9b80decdbbb5df69ec02fb1c702f61082ea8e8",
		"",
	}, "\n")
	if err := os.WriteFile(metadataFile, []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCollector(Config{MetadataFile: metadataFile, Debug: io.Discard}, fakeRunner{
		outputs: map[string]string{
			"kairos-agent --version": "kairos-agent version v2.30.2",
		},
	})

	got := c.collectNodeBuild()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	labels := []string{
		"ubuntu-26.04-amd64-qemu-k8s",
		"ubuntu",
		"26.04",
		"v4.1.2",
		"v2.30.2",
		"k3s",
		"v1.32.6+k3s1",
		"k8s",
		"amd64",
		"qemu",
		"ea9b80decdbbb5df69ec02fb1c702f61082ea8e8",
	}
	assertSample(t, got.samples, c.nodeBuildInfo, labels, 1)

	wantLine := `k2_node_build_info{target="ubuntu-26.04-amd64-qemu-k8s",flavor="ubuntu",flavor_version="26.04",kairos_version="v4.1.2",kairos_agent_version="v2.30.2",kubernetes_distro="k3s",kubernetes_version="v1.32.6+k3s1",role="k8s",arch="amd64",hardware="qemu",source_commit="ea9b80decdbbb5df69ec02fb1c702f61082ea8e8"} 1`
	if rendered := c.Render(); !strings.Contains(rendered, wantLine+"\n") {
		t.Fatalf("rendered exposition missing %q:\n%s", wantLine, rendered)
	} else if !strings.Contains(rendered, `k2_collector_success{collector="node_build"} 1`) {
		t.Fatalf("rendered exposition missing successful node_build collector:\n%s", rendered)
	}
}

func TestCollectNodeBuildStorageWithoutKubernetes(t *testing.T) {
	metadataFile := filepath.Join(t.TempDir(), "metadata.yaml")
	metadata := "target: ubuntu-26.04-amd64-qemu-storage\n" +
		"flavor: ubuntu-26.04\n" +
		"kairosVersion: v4.1.2\n" +
		"kubernetesDistro:   \n" +
		"kubernetesVersion:\n" +
		"role: storage\n" +
		"arch: amd64\n" +
		"hardware: qemu\n" +
		"sourceCommit: ea9b80de\n"
	if err := os.WriteFile(metadataFile, []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCollector(Config{MetadataFile: metadataFile, Debug: io.Discard}, fakeRunner{
		outputs: map[string]string{
			"kairos-agent --version": "kairos-agent version v2.30.2",
		},
	})

	got := c.collectNodeBuild()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	assertSample(t, got.samples, c.nodeBuildInfo, []string{
		"ubuntu-26.04-amd64-qemu-storage", "ubuntu", "26.04", "v4.1.2", "v2.30.2", "", "", "storage", "amd64", "qemu", "ea9b80de",
	}, 1)
}

func TestCollectNodeBuildWithoutKairosAgent(t *testing.T) {
	metadataFile := filepath.Join(t.TempDir(), "metadata.yaml")
	metadata := "target: ubuntu-26.04-amd64-qemu-storage\n" +
		"flavor: ubuntu-26.04\n" +
		"role: storage\n" +
		"arch: amd64\n" +
		"hardware: qemu\n"
	if err := os.WriteFile(metadataFile, []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCollector(Config{MetadataFile: metadataFile, Debug: io.Discard}, fakeRunner{
		outErrs: map[string]error{
			"kairos-agent --version": errors.New("kairos-agent not found"),
		},
	})

	got := c.collectNodeBuild()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	assertSample(t, got.samples, c.nodeBuildInfo, []string{
		"ubuntu-26.04-amd64-qemu-storage", "ubuntu", "26.04", "", "", "", "", "storage", "amd64", "qemu", "",
	}, 1)
}

func TestCollectNodeBuildMissingOrUnreadableMetadata(t *testing.T) {
	tests := map[string]string{
		"missing":    filepath.Join(t.TempDir(), "missing.yaml"),
		"unreadable": t.TempDir(),
	}
	for name, metadataFile := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewCollector(Config{MetadataFile: metadataFile, Debug: io.Discard}, fakeRunner{})

			got := c.collectNodeBuild()
			if got.success {
				t.Fatal("success = true, want false")
			}
			assertNoSample(t, got.samples, c.nodeBuildInfo, nil)

			rendered := c.Render()
			if strings.Contains(rendered, "k2_node_build_info") {
				t.Fatalf("rendered exposition contains build_info sample:\n%s", rendered)
			}
			if !strings.Contains(rendered, `k2_collector_success{collector="node_build"} 0`) {
				t.Fatalf("rendered exposition missing failed node_build collector:\n%s", rendered)
			}
		})
	}
}

func TestCollectNodeBuildWithoutTarget(t *testing.T) {
	metadataFile := filepath.Join(t.TempDir(), "metadata.yaml")
	if err := os.WriteFile(metadataFile, []byte("flavor: ubuntu-26.04\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCollector(Config{MetadataFile: metadataFile, Debug: io.Discard}, fakeRunner{})

	got := c.collectNodeBuild()
	if got.success {
		t.Fatal("success = true, want false")
	}
	assertNoSample(t, got.samples, c.nodeBuildInfo, nil)
}

func TestRenderStorageRoleIncludesStorageCollectors(t *testing.T) {
	metadataFile := testRoleMetadata(t, "storage")
	c := NewCollector(Config{
		ConfigFSRoot: t.TempDir(),
		MetadataFile: metadataFile,
		Debug:        io.Discard,
	}, fakeRunner{outputs: map[string]string{
		"smartctl --scan -j": `{"devices":[]}`,
	}})

	rendered := c.Render()
	for _, collector := range []string{
		"node_build",
		"zfs_pools",
		"zfs_keystatus",
		"zfs_volumes",
		"zfs_snapshots",
		"lio",
		"smart",
		"storage_health",
	} {
		want := `k2_collector_success{collector="` + collector + `"} 1`
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered exposition missing %q:\n%s", want, rendered)
		}
	}
}

func TestRenderK8sRoleSkipsStorageCollectors(t *testing.T) {
	c := NewCollector(Config{
		MetadataFile: testRoleMetadata(t, "k8s"),
		Debug:        io.Discard,
	}, fakeRunner{})

	rendered := c.Render()
	if !strings.Contains(rendered, "k2_node_build_info{") || !strings.Contains(rendered, `role="k8s"`) {
		t.Fatalf("rendered exposition missing k8s node build info:\n%s", rendered)
	}
	for _, collector := range []string{
		"zfs_pools",
		"zfs_keystatus",
		"zfs_volumes",
		"zfs_snapshots",
		"lio",
		"smart",
		"storage_health",
	} {
		unexpected := `k2_collector_success{collector="` + collector + `"}`
		if strings.Contains(rendered, unexpected) {
			t.Fatalf("rendered exposition contains skipped collector %q:\n%s", collector, rendered)
		}
	}
	for _, prefix := range []string{"k2_zfs_", "k2_lio_", "k2_smart_", "k2_storage_"} {
		if strings.Contains(rendered, prefix) {
			t.Fatalf("rendered exposition contains storage-only metric prefix %q:\n%s", prefix, rendered)
		}
	}
}

func TestRenderAndTextfileWrite(t *testing.T) {
	status := filepath.Join(t.TempDir(), "status")
	if err := os.WriteFile(status, []byte("healthy: ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configRoot := t.TempDir()
	saveConfig := filepath.Join(t.TempDir(), "saveconfig.json")
	if err := os.WriteFile(saveConfig, []byte(`{"targets":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCollector(Config{
		ConfigFSRoot: configRoot,
		SaveConfig:   saveConfig,
		StatusFile:   status,
		MetadataFile: testRoleMetadata(t, "storage"),
	}, fakeRunner{
		outputs: map[string]string{
			"zpool list -Hp -o name,size,alloc,frag,health":                             "tank\t1000\t250\t0\tONLINE",
			"zpool list -Hp -o name":                                                    "tank",
			"zfs get -Hp -o name,value keystatus -r -t filesystem tank":                 "tank\tavailable",
			"zfs list -Hp -t volume -o name,volsize,used,usedbydataset,usedbysnapshots": "tank/vol1\t1073741824\t4096\t3072\t1024",
			"smartctl --scan -j":                                                        `{"devices":[]}`,
		},
	})
	body := c.Render()
	for _, want := range []string{
		"# HELP k2_zfs_pool_health",
		"# TYPE k2_zfs_pool_health gauge",
		`k2_zfs_pool_health{pool="tank"} 1`,
		`k2_zfs_keystatus_available{dataset="tank"} 1`,
		`k2_storage_healthy 1`,
		`k2_collector_success{collector="zfs_pools"} 1`,
		`k2_zfs_volume_size_bytes{volume="tank/vol1"} 1.073741824e+09`,
		`k2_zfs_volume_dataset_used_bytes{volume="tank/vol1"} 3072`,
		`k2_zfs_volume_snapshot_used_bytes{volume="tank/vol1"} 1024`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered exposition missing %q:\n%s", want, body)
		}
	}

	// Atomic write: final file has the content, no temp debris remains —
	// the node_exporter textfile collector must never see a partial file.
	dir := t.TempDir()
	path := filepath.Join(dir, "k2.prom")
	if err := writeTextfile(path, body); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatal("textfile content does not match rendered exposition")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("textfile dir has %d entries, want 1 (temp file leaked?)", len(entries))
	}
}

func TestRenderExpositionMultipleLabels(t *testing.T) {
	buildInfo := &desc{
		name:   "k2_node_build_info",
		help:   "K2 node image build information.",
		labels: []string{"flavor", "flavor_version"},
	}
	want := "# HELP k2_node_build_info K2 node image build information.\n" +
		"# TYPE k2_node_build_info gauge\n" +
		"k2_node_build_info{flavor=\"ubuntu\",flavor_version=\"26.04\"} 1\n"

	got := renderExposition([]sample{{
		desc:   buildInfo,
		value:  1,
		labels: []string{"ubuntu", "26.04"},
	}})
	if got != want {
		t.Fatalf("renderExposition() = %q, want %q", got, want)
	}
}

func TestRenderExpositionPreservesSingleAndUnlabeledFamilies(t *testing.T) {
	poolHealth := &desc{
		name:   "k2_zfs_pool_health",
		help:   "ZFS pool health, 1 when ONLINE.",
		labels: []string{"pool"},
	}
	storageHealthy := &desc{
		name: "k2_storage_healthy",
		help: "K2 storage health status.",
	}
	want := "# HELP k2_storage_healthy K2 storage health status.\n" +
		"# TYPE k2_storage_healthy gauge\n" +
		"k2_storage_healthy 1\n" +
		"# HELP k2_zfs_pool_health ZFS pool health, 1 when ONLINE.\n" +
		"# TYPE k2_zfs_pool_health gauge\n" +
		"k2_zfs_pool_health{pool=\"tank\"} 1\n"

	got := renderExposition([]sample{
		{desc: poolHealth, value: 1, labels: []string{"tank"}},
		{desc: storageHealthy, value: 1},
	})
	if got != want {
		t.Fatalf("renderExposition() = %q, want %q", got, want)
	}
}

func TestRenderExpositionSkipsMismatchedLabels(t *testing.T) {
	buildInfo := &desc{
		name:   "k2_node_build_info",
		help:   "K2 node image build information.",
		labels: []string{"flavor", "flavor_version"},
	}
	want := "# HELP k2_node_build_info K2 node image build information.\n" +
		"# TYPE k2_node_build_info gauge\n"

	got := renderExposition([]sample{{
		desc:   buildInfo,
		value:  1,
		labels: []string{"ubuntu"},
	}})
	if got != want {
		t.Fatalf("renderExposition() = %q, want %q", got, want)
	}
}

func testCollector(run fakeRunner) *Collector {
	return NewCollector(Config{Debug: io.Discard}, run)
}

func testRoleMetadata(t *testing.T, role string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	data := "target: ubuntu-26.04-amd64-qemu-" + role + "\n" +
		"flavor: ubuntu-26.04\n" +
		"role: " + role + "\n" +
		"arch: amd64\n" +
		"hardware: qemu\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertSample(t *testing.T, samples []sample, desc any, labels []string, value float64) {
	t.Helper()
	for _, sample := range samples {
		if sample.desc == desc && sameStrings(sample.labels, labels) && sample.value == value {
			return
		}
	}
	t.Fatalf("missing sample labels=%v value=%v", labels, value)
}

func assertNoSample(t *testing.T, samples []sample, desc any, labels []string) {
	t.Helper()
	if hasSample(samples, desc, labels) {
		t.Fatalf("unexpected sample labels=%v", labels)
	}
}

func hasSample(samples []sample, desc any, labels []string) bool {
	for _, sample := range samples {
		if sample.desc == desc && sameStrings(sample.labels, labels) {
			return true
		}
	}
	return false
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type fakeRunner struct {
	outputs map[string]string
	outErrs map[string]error
}

func (r fakeRunner) Run(string, ...string) error {
	return nil
}

func (r fakeRunner) Output(name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	return r.outputs[key], r.outErrs[key]
}

// zpool -Hp prints frag/cap as integer percent; 0 and 1 percent must map
// to 0.00 and 0.01 — a magnitude heuristic once misread 1% as a 1.0 ratio.
func TestParseRatioAlwaysTreatsInputAsPercent(t *testing.T) {
	tests := map[string]float64{"0": 0, "1": 0.01, "42": 0.42, "100": 1, "5%": 0.05}
	for in, want := range tests {
		got, ok := parseRatio(in)
		if !ok || got != want {
			t.Fatalf("parseRatio(%q) = %v/%v, want %v", in, got, ok, want)
		}
	}
	if _, ok := parseRatio("-"); ok {
		t.Fatal("parseRatio accepted non-numeric input")
	}
}

func TestCollectZFSSnapshots(t *testing.T) {
	c := testCollector(fakeRunner{
		outputs: map[string]string{
			"zfs list -Hp -t snapshot -o name,creation": "tank/csi/k2@k2-hourly-20260809T170000Z\t1754758800\n" +
				"tank/csi/k2/pvc-abc@k2-hourly-20260809T170000Z\t1754758800\n" +
				"tank/csi/k2@k2-hourly-20260809T180000Z\t1754762400\n" +
				"tank/csi/k2@k2-daily-20260809T000000Z\t1754697600\n" +
				"tank/csi/k2@migrate\t1754000000",
		},
	})

	got := c.collectZFSSnapshots()
	if !got.success {
		t.Fatal("success = false, want true")
	}
	// Recursive child rows share the suffix: 2 distinct hourly points, not 3.
	assertSample(t, got.samples, c.snapshotCount, []string{"k2-hourly"}, 2)
	assertSample(t, got.samples, c.snapshotLast, []string{"k2-hourly"}, 1754762400)
	assertSample(t, got.samples, c.snapshotCount, []string{"k2-daily"}, 1)
	assertSample(t, got.samples, c.snapshotLast, []string{"k2-daily"}, 1754697600)
	assertNoSample(t, got.samples, c.snapshotCount, []string{"migrate"})
}

func TestCollectZFSSnapshotsMalformedLine(t *testing.T) {
	c := testCollector(fakeRunner{
		outputs: map[string]string{
			"zfs list -Hp -t snapshot -o name,creation": "tank@k2-hourly-20260809T180000Z\t1754762400\nnot-a-snapshot-line",
		},
	})
	got := c.collectZFSSnapshots()
	if got.success {
		t.Fatal("success = true, want false for malformed line")
	}
	assertSample(t, got.samples, c.snapshotCount, []string{"k2-hourly"}, 1)
}

// realistic GRUB env block: key=value lines plus '#' padding to a fixed size.
func writeGRUBEnv(t *testing.T, enabled string, tentative string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "boot_assessment")
	body := "# GRUB Environment Block\n" +
		"# WARNING: Do not edit this file by tools other than grub-editenv!!!\n" +
		"enable_boot_assessment=" + enabled + "\n" +
		"boot_assessment_tentative=" + tentative + "\n"
	body += strings.Repeat("#", 1024-len(body))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The k2-pi-35d9 case: sentinels left set by an upgrade whose success never
// cleared them. The node looks healthy; its next reboot rolls it back.
func TestBootAssessmentArmedSentinelIsReported(t *testing.T) {
	c := NewCollector(Config{
		MetadataFile:       testRoleMetadata(t, "k8s"),
		BootAssessmentFile: writeGRUBEnv(t, "yes", "yes"),
		RunCosDir:          t.TempDir(),
		Debug:              io.Discard,
	}, fakeRunner{})

	rendered := c.Render()
	for _, want := range []string{
		"k2_boot_assessment_armed 1",
		"k2_boot_assessment_enabled 1",
		"k2_boot_slot_passive 0",
		"k2_boot_upgrade_failure 0",
		`k2_collector_success{collector="boot_assessment"} 1`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered exposition missing %q:\n%s", want, rendered)
		}
	}
}

func TestBootAssessmentClearedSentinelsReadZero(t *testing.T) {
	c := NewCollector(Config{
		MetadataFile:       testRoleMetadata(t, "k8s"),
		BootAssessmentFile: writeGRUBEnv(t, "", ""),
		RunCosDir:          t.TempDir(),
		Debug:              io.Discard,
	}, fakeRunner{})

	if rendered := c.Render(); !strings.Contains(rendered, "k2_boot_assessment_armed 0") {
		t.Fatalf("cleared sentinels should report armed 0:\n%s", rendered)
	}
}

// A rollback that already happened: /run/cos carries both markers.
func TestBootAssessmentReportsPassiveBootAndUpgradeFailure(t *testing.T) {
	runCos := t.TempDir()
	for _, name := range []string{"passive_mode", "upgrade_failure"} {
		if err := os.WriteFile(filepath.Join(runCos, name), []byte("1"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := NewCollector(Config{
		MetadataFile:       testRoleMetadata(t, "storage"),
		BootAssessmentFile: writeGRUBEnv(t, "", ""),
		RunCosDir:          runCos,
		Debug:              io.Discard,
	}, fakeRunner{})

	rendered := c.Render()
	for _, want := range []string{"k2_boot_slot_passive 1", "k2_boot_upgrade_failure 1"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered exposition missing %q:\n%s", want, rendered)
		}
	}
}

// An unreadable env block must not read as "unarmed" — we cannot tell, and a
// reassuring zero is worse than an explicit collector failure.
func TestBootAssessmentMissingFileFailsTheGroup(t *testing.T) {
	c := NewCollector(Config{
		MetadataFile:       testRoleMetadata(t, "k8s"),
		BootAssessmentFile: filepath.Join(t.TempDir(), "absent"),
		RunCosDir:          t.TempDir(),
		Debug:              io.Discard,
	}, fakeRunner{})

	rendered := c.Render()
	if !strings.Contains(rendered, `k2_collector_success{collector="boot_assessment"} 0`) {
		t.Fatalf("missing env block should fail the group:\n%s", rendered)
	}
	if strings.Contains(rendered, "k2_boot_assessment_armed") {
		t.Fatalf("failed group must not emit a reassuring armed value:\n%s", rendered)
	}
}
