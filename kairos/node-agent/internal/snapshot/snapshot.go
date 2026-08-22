// Package snapshot implements the appliance-side backup cadence: recursive
// ZFS snapshots of the CSI parent dataset with prefix-scoped retention.
// Pruning stays here (source-side); the NAS pull replica never needs destroy
// rights on the appliance.
package snapshot

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/wyvernzora/k2/kairos/node-agent/internal/runner"
)

type Config struct {
	Dataset  string
	Prefix   string
	Keep     int
	LockPath string
	// Now is injectable for tests; defaults to time.Now.
	Now func() time.Time
	Log io.Writer
}

// Run creates <dataset>@<prefix>-<UTC timestamp> recursively, then destroys
// the oldest snapshots matching the prefix beyond Keep. Snapshot names sort
// chronologically because the timestamp is fixed-width UTC.
func Run(cfg Config, run runner.Runner) (retErr error) {
	if cfg.Dataset == "" || cfg.Prefix == "" || cfg.LockPath == "" {
		return fmt.Errorf("dataset, prefix, and lock path are required")
	}
	if cfg.Keep < 1 {
		return fmt.Errorf("keep must be >= 1, got %d", cfg.Keep)
	}
	now := time.Now
	if cfg.Now != nil {
		now = cfg.Now
	}
	logf := func(format string, args ...any) {
		if cfg.Log != nil {
			fmt.Fprintf(cfg.Log, "k2-snapshot: "+format+"\n", args...)
		}
	}

	logf("start dataset=%s prefix=%s keep=%d", cfg.Dataset, cfg.Prefix, cfg.Keep)
	lock, err := acquireFileLock(cfg.LockPath, logf)
	if err != nil {
		return err
	}
	defer func() {
		if err := lock.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("release snapshot lock: %w", err))
		}
	}()
	logf("acquired lock path=%s", cfg.LockPath)

	snapshots, err := listSnapshots(run, cfg.Dataset)
	if err != nil {
		return err
	}
	name, err := nextSnapshotName(cfg.Dataset, cfg.Prefix, now(), snapshots)
	if err != nil {
		return err
	}

	logf("creating %s", name)
	// `-o democratic-csi:managed_resource=false` is load-bearing, not cosmetic.
	// democratic-csi sets that property local=true on every zvol it provisions,
	// and ZFS user properties INHERIT to snapshots — so a cadence snapshot of a
	// CSI volume looks to the driver like a CSI-managed snapshot. Its
	// DeleteVolume then refuses with "filesystem has dependent snapshots" and
	// retries forever, leaving the PV Released and the zvol leaked. Setting the
	// property local=false here makes the cadence snapshots invisible to that
	// check, so DeleteVolume proceeds to `zfs destroy -R` and removes the volume
	// with its snapshots. Off-box copies on the NAS are the safety net.
	if err := run.Run("zfs", "snapshot", "-r", "-o", "democratic-csi:managed_resource=false", name); err != nil {
		return fmt.Errorf("create snapshot %s: %w", name, err)
	}

	matching := prefixSnapshots(snapshots, cfg.Prefix)
	matching = append(matching, name)
	sort.Strings(matching)
	prune := prunable(matching, cfg.Keep)
	logf("retention: %d matching, keeping %d, pruning %d", len(matching), min(len(matching), cfg.Keep), len(prune))
	// One undestroyable victim (a `zfs hold` taken by the NAS pull identity,
	// a busy clone) must not abandon the older ones behind it: that would
	// grow the snapshot tree by one per tick until the block clears. Destroy
	// every victim, then report the failures together.
	var errs []error
	for _, victim := range prune {
		logf("destroying %s", victim)
		if err := run.Run("zfs", "destroy", "-r", victim); err != nil {
			logf("destroy failed for %s: %v", victim, err)
			errs = append(errs, fmt.Errorf("destroy snapshot %s: %w", victim, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	logf("done")
	return nil
}

// listSnapshots returns the dataset's own snapshots, not descendants'.
// Recursive create and destroy handle children atomically through the parent.
func listSnapshots(run runner.Runner, dataset string) ([]string, error) {
	out, err := run.Output("zfs", "list", "-H", "-t", "snapshot", "-o", "name", "-d", "1", dataset)
	if err != nil {
		return nil, fmt.Errorf("list snapshots of %s: %w", dataset, err)
	}
	snapshots := []string{}
	for line := range strings.Lines(out) {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		snapshots = append(snapshots, name)
	}
	return snapshots, nil
}

func nextSnapshotName(dataset, prefix string, now time.Time, snapshots []string) (string, error) {
	timestamp := now.UTC().Truncate(time.Second)
	latest, found, err := latestManagedSnapshotTime(snapshots)
	if err != nil {
		return "", err
	}
	if found && timestamp.Before(latest) {
		return "", fmt.Errorf(
			"clock is behind latest managed snapshot: now=%s latest=%s",
			timestamp.Format(time.RFC3339),
			latest.Format(time.RFC3339),
		)
	}
	if found && timestamp.Equal(latest) {
		timestamp = latest.Add(time.Second)
	}
	return fmt.Sprintf("%s@%s-%s", dataset, prefix, timestamp.Format("20060102T150405Z")), nil
}

func latestManagedSnapshotTime(snapshots []string) (time.Time, bool, error) {
	var latest time.Time
	found := false
	for _, name := range snapshots {
		_, snap, hasSnapshot := strings.Cut(name, "@")
		if !hasSnapshot {
			continue
		}
		timestamp, managed := managedTimestamp(snap)
		if !managed {
			continue
		}
		parsed, err := time.Parse("20060102T150405Z", timestamp)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("parse managed snapshot timestamp %q: %w", name, err)
		}
		if !found || parsed.After(latest) {
			latest = parsed
			found = true
		}
	}
	return latest, found, nil
}

func managedTimestamp(snapshot string) (string, bool) {
	for _, prefix := range []string{"k2-hourly-", "k2-daily-"} {
		if timestamp, found := strings.CutPrefix(snapshot, prefix); found {
			return timestamp, true
		}
	}
	return "", false
}

func prefixSnapshots(snapshots []string, prefix string) []string {
	matching := []string{}
	for _, name := range snapshots {
		_, snap, found := strings.Cut(name, "@")
		if found && strings.HasPrefix(snap, prefix+"-") {
			matching = append(matching, name)
		}
	}
	return matching
}

func prunable(sorted []string, keep int) []string {
	if len(sorted) <= keep {
		return nil
	}
	return sorted[:len(sorted)-keep]
}
