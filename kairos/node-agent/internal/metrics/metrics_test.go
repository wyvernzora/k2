package metrics

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectZFSPools(t *testing.T) {
	c := testCollector(fakeRunner{
		outputs: map[string]string{
			"zpool list -Hp -o name,size,alloc,frag,cap,health": strings.Join([]string{
				"tank\t1000\t250\t12\t25\tONLINE",
				"bad line",
			}, "\n"),
		},
	})

	got := c.collectZFSPools()
	if got.success {
		t.Fatal("success = true, want false for malformed line")
	}
	assertSample(t, got.samples, c.zfsPoolHealth, []string{"tank"}, 1)
	assertSample(t, got.samples, c.zfsPoolSize, []string{"tank"}, 1000)
	assertSample(t, got.samples, c.zfsPoolAlloc, []string{"tank"}, 250)
	assertSample(t, got.samples, c.zfsPoolFrag, []string{"tank"}, 0.12)
	assertSample(t, got.samples, c.zfsPoolCap, []string{"tank"}, 0.25)
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
			"zfs list -Hp -t volume -o name,volsize,used": strings.Join([]string{
				"tank/vol1\t1073741824\t4096",
				"broken",
			}, "\n"),
		},
	})

	got := c.collectZFSVolumes()
	if got.success {
		t.Fatal("success = true, want false for malformed line")
	}
	assertSample(t, got.samples, c.zfsVolumeSize, []string{"tank/vol1"}, 1073741824)
	assertSample(t, got.samples, c.zfsVolumeUsed, []string{"tank/vol1"}, 4096)
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

func TestMetricsHandler(t *testing.T) {
	status := filepath.Join(t.TempDir(), "status")
	if err := os.WriteFile(status, []byte("healthy: ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configRoot := t.TempDir()
	saveConfig := filepath.Join(t.TempDir(), "saveconfig.json")
	if err := os.WriteFile(saveConfig, []byte(`{"targets":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := Handler(Config{ConfigFSRoot: configRoot, SaveConfig: saveConfig, StatusFile: status}, fakeRunner{
		outputs: map[string]string{
			"zpool list -Hp -o name,size,alloc,frag,cap,health":         "tank\t1000\t250\t0\t25\tONLINE",
			"zpool list -Hp -o name":                                    "tank",
			"zfs get -Hp -o name,value keystatus -r -t filesystem tank": "tank\tavailable",
			"zfs list -Hp -t volume -o name,volsize,used":               "tank/vol1\t1073741824\t4096",
			"smartctl --scan -j":                                        `{"devices":[]}`,
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`k2_zfs_pool_health{pool="tank"} 1`,
		`k2_zfs_keystatus_available{dataset="tank"} 1`,
		`k2_storage_healthy 1`,
		`k2_collector_success{collector="zfs_pools"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}

func testCollector(run fakeRunner) *Collector {
	return NewCollector(Config{Debug: io.Discard}, run)
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
