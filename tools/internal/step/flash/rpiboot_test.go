package flash

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fakePlatform exists only for testing WaitForNewDisks; it returns a
// scripted sequence of disk snapshots, one per Enumerate call.
type fakePlatform struct {
	snapshots [][]Disk
	idx       atomic.Int64
}

func (f *fakePlatform) EnumerateExternalDisks(ctx context.Context) ([]Disk, error) {
	i := f.idx.Add(1) - 1
	if int(i) >= len(f.snapshots) {
		// After the script is exhausted, keep returning the last snapshot.
		return f.snapshots[len(f.snapshots)-1], nil
	}
	return f.snapshots[i], nil
}

func (f *fakePlatform) DiskByID(ctx context.Context, id string) (Disk, error) {
	return Disk{}, errors.New("not used in tests")
}

func (f *fakePlatform) Unmount(ctx context.Context, d Disk) error { return nil }
func (f *fakePlatform) Eject(ctx context.Context, d Disk) error   { return nil }

func TestWaitForNewDisksReturnsOnceStabilized(t *testing.T) {
	before := []Disk{{ID: "disk0"}}
	fp := &fakePlatform{
		snapshots: [][]Disk{
			// Poll 1: nothing new yet (still booting).
			{{ID: "disk0"}},
			// Poll 2: eMMC shows up.
			{{ID: "disk0"}, {ID: "disk4", SizeBytes: 32 * gib}},
			// Poll 3: same → diff stable → return.
			{{ID: "disk0"}, {ID: "disk4", SizeBytes: 32 * gib}},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := WaitForNewDisks(ctx, fp, before, 5*time.Second)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if len(got) != 1 || got[0].ID != "disk4" {
		t.Fatalf("got %+v", got)
	}
}

func TestWaitForNewDisksTimesOutWhenNothingAppears(t *testing.T) {
	before := []Disk{{ID: "disk0"}}
	fp := &fakePlatform{
		snapshots: [][]Disk{
			{{ID: "disk0"}},
			{{ID: "disk0"}},
		},
	}
	ctx := context.Background()
	_, err := WaitForNewDisks(ctx, fp, before, 50*time.Millisecond)
	if !errors.Is(err, ErrRpibootTimeout) {
		t.Fatalf("err = %v, want ErrRpibootTimeout", err)
	}
}

func TestWaitForNewDisksRespectsContextCancel(t *testing.T) {
	before := []Disk{{ID: "disk0"}}
	fp := &fakePlatform{snapshots: [][]Disk{{{ID: "disk0"}}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := WaitForNewDisks(ctx, fp, before, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
