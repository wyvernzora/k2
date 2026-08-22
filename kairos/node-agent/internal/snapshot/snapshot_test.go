package snapshot

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	commands [][]string
	listOut  string
	failOn   string
}

type snapshotRecordingRunner struct {
	snapshots []string
}

type concurrentRunner struct {
	mu                 sync.Mutex
	snapshots          []string
	listCount          int
	firstCreateOnce    sync.Once
	firstCreateStarted chan struct{}
	releaseFirstCreate chan struct{}
	secondListStarted  chan struct{}
}

type channelWriter struct {
	writes chan string
}

func (r *snapshotRecordingRunner) Run(name string, args ...string) error {
	if name == "zfs" && len(args) > 0 && args[0] == "snapshot" {
		r.snapshots = append(r.snapshots, args[len(args)-1])
	}
	return nil
}

func (r *snapshotRecordingRunner) Output(string, ...string) (string, error) {
	return strings.Join(r.snapshots, "\n"), nil
}

func (r *concurrentRunner) Run(name string, args ...string) error {
	if name != "zfs" || len(args) == 0 || args[0] != "snapshot" {
		return nil
	}

	r.mu.Lock()
	isFirst := len(r.snapshots) == 0
	r.mu.Unlock()
	if isFirst {
		r.firstCreateOnce.Do(func() { close(r.firstCreateStarted) })
		<-r.releaseFirstCreate
	}

	r.mu.Lock()
	r.snapshots = append(r.snapshots, args[len(args)-1])
	r.mu.Unlock()
	return nil
}

func (r *concurrentRunner) Output(string, ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listCount++
	if r.listCount == 2 {
		close(r.secondListStarted)
	}
	return strings.Join(r.snapshots, "\n"), nil
}

func (w channelWriter) Write(p []byte) (int, error) {
	w.writes <- string(p)
	return len(p), nil
}

func (f *fakeRunner) Run(name string, args ...string) error {
	cmd := append([]string{name}, args...)
	f.commands = append(f.commands, cmd)
	joined := strings.Join(cmd, " ")
	if f.failOn != "" && strings.Contains(joined, f.failOn) {
		return fmt.Errorf("forced failure on %q", joined)
	}
	return nil
}

func (f *fakeRunner) Output(name string, args ...string) (string, error) {
	f.commands = append(f.commands, append([]string{name}, args...))
	return f.listOut, nil
}

func fixedNow() time.Time {
	return time.Date(2026, 8, 9, 18, 0, 0, 0, time.UTC)
}

func testConfig(t *testing.T, prefix string, keep int) Config {
	t.Helper()
	return Config{
		Dataset:  "tank/csi/k2",
		Prefix:   prefix,
		Keep:     keep,
		LockPath: filepath.Join(t.TempDir(), "snapshot.lock"),
		Now:      fixedNow,
	}
}

func TestRunUsesStrictlyIncreasingManagedTimestamps(t *testing.T) {
	run := &snapshotRecordingRunner{}
	cfg := testConfig(t, "k2-daily", 30)
	for _, prefix := range []string{"k2-daily", "k2-hourly"} {
		cfg.Prefix = prefix
		err := Run(cfg, run)
		if err != nil {
			t.Fatalf("Run(%q): %v", prefix, err)
		}
	}

	want := []string{
		"tank/csi/k2@k2-daily-20260809T180000Z",
		"tank/csi/k2@k2-hourly-20260809T180001Z",
	}
	if strings.Join(run.snapshots, "\n") != strings.Join(want, "\n") {
		t.Fatalf("snapshots = %v, want %v", run.snapshots, want)
	}
}

func TestRunSerializesConcurrentCadences(t *testing.T) {
	run := &concurrentRunner{
		firstCreateStarted: make(chan struct{}),
		releaseFirstCreate: make(chan struct{}),
		secondListStarted:  make(chan struct{}),
	}
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(run.releaseFirstCreate) })
	})

	lockPath := filepath.Join(t.TempDir(), "snapshot.lock")
	daily := testConfig(t, "k2-daily", 30)
	daily.LockPath = lockPath
	hourly := testConfig(t, "k2-hourly", 48)
	hourly.LockPath = lockPath
	logWrites := make(chan string, 16)
	hourly.Log = channelWriter{writes: logWrites}

	errs := make(chan error, 2)
	go func() { errs <- Run(daily, run) }()
	waitForSignal(t, run.firstCreateStarted, "first snapshot creation")
	go func() { errs <- Run(hourly, run) }()
	waitForLog(t, logWrites, "waiting for lock")

	select {
	case <-run.secondListStarted:
		t.Fatal("second cadence listed snapshots before the first cadence released the lock")
	default:
	}

	releaseOnce.Do(func() { close(run.releaseFirstCreate) })
	for range 2 {
		select {
		case err := <-errs:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent snapshot runs")
		}
	}

	want := []string{
		"tank/csi/k2@k2-daily-20260809T180000Z",
		"tank/csi/k2@k2-hourly-20260809T180001Z",
	}
	if strings.Join(run.snapshots, "\n") != strings.Join(want, "\n") {
		t.Fatalf("snapshots = %v, want %v", run.snapshots, want)
	}
}

func TestRunRejectsBackwardClock(t *testing.T) {
	run := &fakeRunner{listOut: "tank/csi/k2@k2-daily-20260809T190000Z\n"}
	err := Run(testConfig(t, "k2-hourly", 48), run)
	if err == nil || !strings.Contains(err.Error(), "clock is behind latest managed snapshot") {
		t.Fatalf("expected backward clock error, got %v", err)
	}
	for _, cmd := range run.commands {
		if len(cmd) > 1 && cmd[1] == "snapshot" {
			t.Fatalf("created snapshot with a backward clock: %v", cmd)
		}
	}
}

func TestRunIgnoresManualSnapshotsWhenChoosingTimestamp(t *testing.T) {
	run := &fakeRunner{listOut: "tank/csi/k2@migrate-20990101T000000Z\n" +
		"tank/csi/k2@replication-checkpoint-20990101T000000Z"}
	if err := Run(testConfig(t, "k2-hourly", 48), run); err != nil {
		t.Fatal(err)
	}
	create := strings.Join(run.commands[1], " ")
	if !strings.HasSuffix(create, "tank/csi/k2@k2-hourly-20260809T180000Z") {
		t.Fatalf("manual snapshots changed the managed timestamp: %s", create)
	}
}

func TestRunReleasesLockAfterCreateError(t *testing.T) {
	cfg := testConfig(t, "k2-hourly", 48)
	failed := &fakeRunner{failOn: "snapshot -r"}
	if err := Run(cfg, failed); err == nil {
		t.Fatal("expected create error")
	}

	done := make(chan error, 1)
	go func() { done <- Run(cfg, &fakeRunner{}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second run could not acquire the lock after an error")
	}
}

func waitForSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForLog(t *testing.T, writes <-chan string, substring string) {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case write := <-writes:
			if strings.Contains(write, substring) {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for log containing %q", substring)
		}
	}
}

func TestRunCreatesRecursiveSnapshotAndPrunes(t *testing.T) {
	run := &fakeRunner{listOut: strings.Join([]string{
		"tank/csi/k2@k2-hourly-20260809T150000Z",
		"tank/csi/k2@k2-hourly-20260809T160000Z",
		"tank/csi/k2@k2-hourly-20260809T170000Z",
		"tank/csi/k2@k2-daily-20260809T000000Z", // other cadence: untouched
		"tank/csi/k2@migrate",                   // manual: untouched
	}, "\n")}
	var log strings.Builder
	cfg := testConfig(t, "k2-hourly", 2)
	cfg.Log = &log
	err := Run(cfg, run)
	if err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"zfs", "list", "-H", "-t", "snapshot", "-o", "name", "-d", "1", "tank/csi/k2"},
		{"zfs", "snapshot", "-r", "-o", "democratic-csi:managed_resource=false", "tank/csi/k2@k2-hourly-20260809T180000Z"},
		{"zfs", "destroy", "-r", "tank/csi/k2@k2-hourly-20260809T150000Z"},
		{"zfs", "destroy", "-r", "tank/csi/k2@k2-hourly-20260809T160000Z"},
	}
	if len(run.commands) != len(want) {
		t.Fatalf("commands = %v, want %v", run.commands, want)
	}
	for i := range want {
		if strings.Join(run.commands[i], " ") != strings.Join(want[i], " ") {
			t.Fatalf("command %d = %v, want %v", i, run.commands[i], want[i])
		}
	}
	for _, phase := range []string{"start", "acquired lock", "creating", "retention:", "done"} {
		if !strings.Contains(log.String(), phase) {
			t.Errorf("log missing %q:\n%s", phase, log.String())
		}
	}
}

func TestRunNoPruneUnderKeep(t *testing.T) {
	run := &fakeRunner{listOut: "tank/csi/k2@k2-daily-20260809T000000Z\n"}
	if err := Run(testConfig(t, "k2-daily", 30), run); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range run.commands {
		if cmd[1] == "destroy" {
			t.Fatalf("unexpected destroy: %v", cmd)
		}
	}
}

func TestRunFailsOnCreateError(t *testing.T) {
	run := &fakeRunner{failOn: "snapshot -r"}
	err := Run(testConfig(t, "k2-hourly", 48), run)
	if err == nil || !strings.Contains(err.Error(), "create snapshot") {
		t.Fatalf("expected create error, got %v", err)
	}
}

// A held snapshot must not shield the older ones queued behind it.
func TestRunPrunesRemainingVictimsAfterDestroyFailure(t *testing.T) {
	run := &fakeRunner{
		listOut: strings.Join([]string{
			"tank/csi/k2@k2-hourly-20260809T150000Z",
			"tank/csi/k2@k2-hourly-20260809T160000Z",
			"tank/csi/k2@k2-hourly-20260809T170000Z",
			"tank/csi/k2@k2-hourly-20260809T180000Z",
		}, "\n"),
		failOn: "destroy -r tank/csi/k2@k2-hourly-20260809T150000Z",
	}
	err := Run(testConfig(t, "k2-hourly", 1), run)
	if err == nil || !strings.Contains(err.Error(), "20260809T150000Z") {
		t.Fatalf("expected the failed destroy to be reported, got %v", err)
	}
	for _, want := range []string{"20260809T160000Z", "20260809T170000Z"} {
		found := false
		for _, cmd := range run.commands {
			if cmd[1] == "destroy" && strings.Contains(strings.Join(cmd, " "), want) {
				found = true
			}
		}
		if !found {
			t.Errorf("victim %s was never destroyed: %v", want, run.commands)
		}
	}
}

func TestRunValidatesConfig(t *testing.T) {
	if err := Run(Config{Prefix: "p", Keep: 1}, &fakeRunner{}); err == nil {
		t.Fatal("expected error for missing dataset")
	}
	if err := Run(Config{Dataset: "d", Prefix: "p", LockPath: "lock", Keep: 0}, &fakeRunner{}); err == nil {
		t.Fatal("expected error for keep=0")
	}
	if err := Run(Config{Dataset: "d", Prefix: "p", Keep: 1}, &fakeRunner{}); err == nil {
		t.Fatal("expected error for missing lock path")
	}
}

// democratic-csi sets `democratic-csi:managed_resource=true` local on every
// zvol it provisions, and ZFS user properties inherit to snapshots. Without an
// explicit local=false, a cadence snapshot reads back as CSI-managed and the
// driver's DeleteVolume refuses with "filesystem has dependent snapshots",
// retrying forever and leaking the zvol. Assert the flag is present and false —
// if it is ever dropped, PVC deletion silently breaks for every volume older
// than one snapshot interval.
func TestRunMarksSnapshotsUnmanagedSoCsiCanDeleteTheVolume(t *testing.T) {
	run := &fakeRunner{}
	cfg := testConfig(t, "k2-hourly", 5)
	cfg.Log = &strings.Builder{}
	if err := Run(cfg, run); err != nil {
		t.Fatal(err)
	}
	create := strings.Join(run.commands[1], " ")
	if !strings.Contains(create, "-o democratic-csi:managed_resource=false") {
		t.Fatalf("cadence snapshot must be marked unmanaged or CSI cannot delete the volume; got: %s", create)
	}
}
