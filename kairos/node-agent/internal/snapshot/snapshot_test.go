package snapshot

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	commands [][]string
	listOut  string
	failOn   string
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

func TestRunCreatesRecursiveSnapshotAndPrunes(t *testing.T) {
	run := &fakeRunner{listOut: strings.Join([]string{
		"tank/csi/k2@k2-hourly-20260809T150000Z",
		"tank/csi/k2@k2-hourly-20260809T160000Z",
		"tank/csi/k2@k2-hourly-20260809T170000Z",
		"tank/csi/k2@k2-hourly-20260809T180000Z",
		"tank/csi/k2@k2-daily-20260809T000000Z", // other cadence: untouched
		"tank/csi/k2@migrate",                   // manual: untouched
	}, "\n")}
	var log strings.Builder
	err := Run(Config{Dataset: "tank/csi/k2", Prefix: "k2-hourly", Keep: 2, Now: fixedNow, Log: &log}, run)
	if err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"zfs", "snapshot", "-r", "-o", "democratic-csi:managed_resource=false", "tank/csi/k2@k2-hourly-20260809T180000Z"},
		{"zfs", "list", "-H", "-t", "snapshot", "-o", "name", "-d", "1", "tank/csi/k2"},
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
	for _, phase := range []string{"start", "creating", "retention:", "done"} {
		if !strings.Contains(log.String(), phase) {
			t.Errorf("log missing %q:\n%s", phase, log.String())
		}
	}
}

func TestRunNoPruneUnderKeep(t *testing.T) {
	run := &fakeRunner{listOut: "tank/csi/k2@k2-daily-20260809T000000Z\n"}
	if err := Run(Config{Dataset: "tank/csi/k2", Prefix: "k2-daily", Keep: 30, Now: fixedNow}, run); err != nil {
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
	err := Run(Config{Dataset: "tank/csi/k2", Prefix: "k2-hourly", Keep: 48, Now: fixedNow}, run)
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
	err := Run(Config{Dataset: "tank/csi/k2", Prefix: "k2-hourly", Keep: 1, Now: fixedNow}, run)
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
	if err := Run(Config{Dataset: "d", Prefix: "p", Keep: 0}, &fakeRunner{}); err == nil {
		t.Fatal("expected error for keep=0")
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
	run := &fakeRunner{listOut: "tank/csi/k2@k2-hourly-20260809T180000Z"}
	if err := Run(Config{Dataset: "tank/csi/k2", Prefix: "k2-hourly", Keep: 5, Now: fixedNow, Log: &strings.Builder{}}, run); err != nil {
		t.Fatal(err)
	}
	create := strings.Join(run.commands[0], " ")
	if !strings.Contains(create, "-o democratic-csi:managed_resource=false") {
		t.Fatalf("cadence snapshot must be marked unmanaged or CSI cannot delete the volume; got: %s", create)
	}
}
