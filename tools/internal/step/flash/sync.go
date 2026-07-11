package flash

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// SyncDisks shells out to `sync` so any in-flight writes hit the
// platter (or eMMC) before we proceed to verification.
func SyncDisks(ctx context.Context, progress io.Writer) error {
	cmd := exec.CommandContext(ctx, "sync")
	cmd.Stdout = progress
	cmd.Stderr = progress
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flash: sync: %w", err)
	}
	return nil
}
