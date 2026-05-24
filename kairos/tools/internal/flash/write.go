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

// copyBufferSize is the staging buffer between `xz -dc` stdout and the
// `sudo dd` stdin. 4 MiB matches dd's bs=4m output block so each
// io.CopyBuffer cycle ideally hands dd one full block.
const copyBufferSize = 4 * 1024 * 1024

// ErrXZNotInstalled signals the host doesn't have the `xz` binary on
// PATH. Surfaced with an install hint by the runner.
var ErrXZNotInstalled = errors.New("flash: `xz` binary not found on PATH (install via `brew install xz` on macOS, `apt install xz-utils` on Debian/Ubuntu)")

// WriteRawDisk decompresses `compressedPath` via a native `xz -dc`
// subprocess and streams the resulting bytes into a parallel
// `sudo dd of=<rawPath> bs=4m` subprocess. `progress` receives each
// byte that flows through the pipeline; the Reporter.Progress
// renderer derives bytes/rate/ETA from its own internal counter, so
// `progress` need only implement io.Writer.
//
// Native `xz` is used (rather than a pure-Go decoder) because LZMA2
// decompression is CPU-bound and the native implementation is roughly
// 2× faster than pure Go — the bottleneck for a 4 GiB .raw.xz on
// Apple Silicon.
//
// dd's own stdout/stderr is captured to a small buffer for error
// reporting; we do not surface it during the run because the Progress
// Kind owns the display and intercalating dd's `status=progress`
// chatter into that display would clutter the bar.
func WriteRawDisk(ctx context.Context, progress io.Writer, compressedPath string, totalBytes uint64, rawPath string) error {
	if _, err := exec.LookPath("xz"); err != nil {
		return ErrXZNotInstalled
	}

	xzCmd := exec.CommandContext(ctx, "xz", "-dc", compressedPath)
	xzStdout, err := xzCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("flash: xz stdout pipe: %w", err)
	}
	var xzStderr bytes.Buffer
	xzCmd.Stderr = &xzStderr

	ddCmd := exec.CommandContext(ctx, "sudo", "dd", "of="+rawPath, "bs=4m")
	ddStdin, err := ddCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("flash: dd stdin pipe: %w", err)
	}
	var ddStderr bytes.Buffer
	ddCmd.Stdout = io.Discard
	ddCmd.Stderr = &ddStderr

	if err := xzCmd.Start(); err != nil {
		return fmt.Errorf("flash: start xz: %w", err)
	}
	if err := ddCmd.Start(); err != nil {
		_ = xzCmd.Process.Kill()
		_ = xzCmd.Wait()
		return fmt.Errorf("flash: start dd: %w", err)
	}

	// Tee the decompressed bytes to BOTH dd's stdin (the actual disk
	// write) and the Progress (a pure byte counter). io.MultiWriter
	// preserves dd's hot path — it only adds a counter increment per
	// chunk, which is cheap.
	dst := io.MultiWriter(ddStdin, progress)
	copyErr := streamWithCancel(ctx, dst, xzStdout)

	// Close dd's stdin so it sees EOF and exits cleanly.
	_ = ddStdin.Close()
	xzErr := xzCmd.Wait()
	ddErr := ddCmd.Wait()

	if copyErr != nil {
		return fmt.Errorf("flash: stream decompressed image to dd: %w", copyErr)
	}
	if xzErr != nil {
		return fmt.Errorf("flash: xz exited %w (stderr: %s)", xzErr, strings.TrimSpace(xzStderr.String()))
	}
	if ddErr != nil {
		return fmt.Errorf("flash: dd exited %w (stderr: %s)", ddErr, strings.TrimSpace(ddStderr.String()))
	}
	return nil
}

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

// streamWithCancel is io.CopyBuffer with context cancellation, sized
// to match `dd bs=4m` so every cycle hands dd close to a full output
// block.
func streamWithCancel(ctx context.Context, dst io.Writer, src io.Reader) error {
	done := make(chan error, 1)
	buf := make([]byte, copyBufferSize)
	go func() {
		_, err := io.CopyBuffer(dst, src, buf)
		done <- err
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

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

// ErrHashMismatch indicates the post-write readback didn't match the
// manifest's raw.sha256.
var ErrHashMismatch = errors.New("flash: hash mismatch after write")

// chooseReadBlocks picks a (bs, count) pair such that bs*count
// exactly equals expectedSize. The build pipeline writes 1-MiB-aligned
// images, so the 1-MiB branch is the hot path.
func chooseReadBlocks(size uint64) (block, count uint64, err error) {
	const mib = uint64(1024 * 1024)
	const sector = uint64(512)
	switch {
	case size%mib == 0:
		return mib, size / mib, nil
	case size%sector == 0:
		return sector, size / sector, nil
	default:
		return 0, 0, fmt.Errorf("flash: image size %d is not aligned to 512 B or 1 MiB; verification would over/under-read", size)
	}
}
