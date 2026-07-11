package flash

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// VerifyRawDisk reads the first `expectedSize` bytes from `rawPath`
// via `sudo dd | sha256` and compares the hex digest to `expectedHex`.
// `progress` receives every byte the verifier reads, for the
// Reporter.Progress bar/rate/ETA. Returns ErrHashMismatch on a digest
// mismatch.
func VerifyRawDisk(ctx context.Context, progress io.Writer, rawPath string, expectedSize uint64, expectedHex string) error {
	if expectedSize == 0 || expectedHex == "" {
		return errors.New("flash: cannot verify without expectedSize + expectedHex")
	}
	blockSize, blockCount, err := chooseReadBlocks(expectedSize)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "sudo", "dd",
		"if="+rawPath,
		fmt.Sprintf("bs=%d", blockSize),
		fmt.Sprintf("count=%d", blockCount),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("flash: dd stdout pipe: %w", err)
	}
	var ddStderr bytes.Buffer
	cmd.Stderr = &ddStderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("flash: start dd for verify: %w", err)
	}

	hasher := sha256.New()
	// Tee dd's stdout to BOTH the hasher and Progress (the byte
	// counter that drives the bar/rate/ETA).
	dst := io.MultiWriter(hasher, progress)
	copyErr := streamWithCancel(ctx, dst, stdout)
	waitErr := cmd.Wait()

	if copyErr != nil {
		return fmt.Errorf("flash: read disk for verify: %w (dd stderr: %s)", copyErr, strings.TrimSpace(ddStderr.String()))
	}
	if waitErr != nil {
		return fmt.Errorf("flash: dd verify exited %w (stderr: %s)", waitErr, strings.TrimSpace(ddStderr.String()))
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(got, expectedHex) {
		return fmt.Errorf("%w: got %s, want %s", ErrHashMismatch, got, expectedHex)
	}
	return nil
}
