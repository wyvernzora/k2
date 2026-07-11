package flash

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// ZeroDiskHead zeros the first `mibCount` MiB of `rawPath`, using
// `sudo dd if=/dev/zero of=<rawPath> bs=1m count=<n>`. Used for wiping
// the NVMe boot/partition header so it doesn't interfere with eMMC boot.
func ZeroDiskHead(ctx context.Context, progress io.Writer, rawPath string, mibCount uint64) error {
	cmd := exec.CommandContext(ctx, "sudo", "dd",
		"if=/dev/zero",
		"of="+rawPath,
		"bs=1m",
		fmt.Sprintf("count=%d", mibCount),
	)
	cmd.Stdout = progress
	cmd.Stderr = progress
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flash: zero head of %s: %w", rawPath, err)
	}
	return nil
}
