package flash

import (
	"context"
	"fmt"
	"time"
)

func WaitForNewDisks(ctx context.Context, p Platform, before []Disk, timeout time.Duration) ([]Disk, error) {
	deadline := time.Now().Add(timeout)
	const pollInterval = time.Second
	var lastDiff []Disk
	lastDiffCount := -1
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		after, err := p.EnumerateExternalDisks(ctx)
		if err != nil {
			return nil, fmt.Errorf("flash: enumerate disks while waiting: %w", err)
		}
		diff := DiffDisks(before, after)
		if len(diff) > 0 && len(diff) == lastDiffCount {
			return diff, nil
		}
		lastDiff, lastDiffCount = diff, len(diff)
		if time.Now().After(deadline) {
			if len(lastDiff) > 0 {
				return lastDiff, nil
			}
			return nil, ErrRpibootTimeout
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}
