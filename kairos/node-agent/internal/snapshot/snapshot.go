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
	Dataset string
	Prefix  string
	Keep    int
	// Now is injectable for tests; defaults to time.Now.
	Now func() time.Time
	Log io.Writer
}

// Run creates <dataset>@<prefix>-<UTC timestamp> recursively, then destroys
// the oldest snapshots matching the prefix beyond Keep. Snapshot names sort
// chronologically because the timestamp is fixed-width UTC.
func Run(cfg Config, run runner.Runner) error {
	if cfg.Dataset == "" || cfg.Prefix == "" {
		return fmt.Errorf("dataset and prefix are required")
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

	name := fmt.Sprintf("%s@%s-%s", cfg.Dataset, cfg.Prefix, now().UTC().Format("20060102T150405Z"))
	logf("start dataset=%s prefix=%s keep=%d", cfg.Dataset, cfg.Prefix, cfg.Keep)
	logf("creating %s", name)
	if err := run.Run("zfs", "snapshot", "-r", name); err != nil {
		return fmt.Errorf("create snapshot %s: %w", name, err)
	}

	matching, err := listPrefixSnapshots(run, cfg.Dataset, cfg.Prefix)
	if err != nil {
		return err
	}
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

// listPrefixSnapshots returns the dataset's own snapshots (not descendants':
// recursive create/destroy handles children atomically through the parent)
// whose names start with "<prefix>-", sorted ascending (oldest first).
func listPrefixSnapshots(run runner.Runner, dataset, prefix string) ([]string, error) {
	out, err := run.Output("zfs", "list", "-H", "-t", "snapshot", "-o", "name", "-d", "1", dataset)
	if err != nil {
		return nil, fmt.Errorf("list snapshots of %s: %w", dataset, err)
	}
	var matching []string
	for line := range strings.Lines(out) {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		_, snap, found := strings.Cut(name, "@")
		if !found || !strings.HasPrefix(snap, prefix+"-") {
			continue
		}
		matching = append(matching, name)
	}
	sort.Strings(matching)
	return matching, nil
}

func prunable(sorted []string, keep int) []string {
	if len(sorted) <= keep {
		return nil
	}
	return sorted[:len(sorted)-keep]
}
