//go:build darwin

package flash

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/wyvernzora/k2/kairos/tools/internal/ui"
)

// darwinPlatform is the macOS Platform implementation. It shells out to
// `diskutil` for enumeration / unmount / eject and to `plutil` to convert
// the plist output to JSON for in-process parsing.
type darwinPlatform struct{}

func currentPlatform() (Platform, error) {
	return darwinPlatform{}, nil
}

// EnumerateExternalDisks returns every external, whole, physical disk
// macOS currently reports. Internal drives (built-in SSD) and partition
// slices are filtered out so we never accidentally surface them to the
// classifier or the operator.
func (p darwinPlatform) EnumerateExternalDisks(ctx context.Context) ([]Disk, error) {
	listJSON, err := pipeDiskutilJSON(ctx, "list", "-plist")
	if err != nil {
		return nil, err
	}
	var listed struct {
		WholeDisks []string `json:"WholeDisks"`
	}
	if err := json.Unmarshal(listJSON, &listed); err != nil {
		return nil, fmt.Errorf("flash: parse diskutil list: %w", err)
	}

	var disks []Disk
	for _, id := range listed.WholeDisks {
		d, err := p.diskInfo(ctx, id)
		if err != nil {
			// A disk can disappear between `diskutil list` and
			// `diskutil info` (USB unplug, eject by another process,
			// etc.). Skip transient failures rather than aborting the
			// whole enumeration.
			continue
		}
		if d.Internal || !d.WholeDisk || !d.Physical {
			continue
		}
		disks = append(disks, d)
	}
	return disks, nil
}

func (p darwinPlatform) DiskByID(ctx context.Context, id string) (Disk, error) {
	d, err := p.diskInfo(ctx, id)
	if err != nil {
		return Disk{}, err
	}
	if d.Internal {
		return Disk{}, fmt.Errorf("flash: refusing to operate on internal disk %s", d.Path)
	}
	if !d.WholeDisk {
		return Disk{}, fmt.Errorf("flash: %s is not a whole disk", d.Path)
	}
	if !d.Physical {
		return Disk{}, fmt.Errorf("flash: %s is not a physical disk", d.Path)
	}
	return d, nil
}

func (p darwinPlatform) Unmount(ctx context.Context, d Disk) error {
	return runDiskutil(ctx, "unmountDisk", d.Path)
}

func (p darwinPlatform) Eject(ctx context.Context, d Disk) error {
	return runDiskutil(ctx, "eject", d.Path)
}

// diskInfo wraps `diskutil info -plist <id>` and parses the relevant
// fields. The plist field set is large; we extract only what the flasher
// uses. The boolean fields below are populated regardless of whether the
// underlying value was the string "Yes"/"No" or a real plist boolean —
// `plutil -convert json` normalizes both to JSON booleans.
func (p darwinPlatform) diskInfo(ctx context.Context, id string) (Disk, error) {
	body, err := pipeDiskutilJSON(ctx, "info", "-plist", id)
	if err != nil {
		return Disk{}, fmt.Errorf("flash: diskutil info %s: %w", id, err)
	}
	var info struct {
		DeviceIdentifier  string `json:"DeviceIdentifier"`
		DeviceNode        string `json:"DeviceNode"`
		Internal          bool   `json:"Internal"`
		WholeDisk         bool   `json:"WholeDisk"`
		VirtualOrPhysical string `json:"VirtualOrPhysical"`
		MediaName         string `json:"MediaName"`
		IORegistryName    string `json:"IORegistryEntryName"`
		Size              uint64 `json:"Size"`
		TotalSize         uint64 `json:"TotalSize"`
		BusProtocol       string `json:"BusProtocol"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return Disk{}, fmt.Errorf("flash: parse diskutil info for %s: %w", id, err)
	}

	size := info.TotalSize
	if size == 0 {
		size = info.Size
	}
	mediaName := info.MediaName
	if mediaName == "" {
		mediaName = info.IORegistryName
	}
	description := mediaName
	if info.BusProtocol != "" {
		description = strings.TrimSpace(fmt.Sprintf("%s (%s, %s)", mediaName, info.BusProtocol, ui.HumanBytes(size)))
	} else {
		description = strings.TrimSpace(fmt.Sprintf("%s (%s)", mediaName, ui.HumanBytes(size)))
	}

	devicePath := info.DeviceNode
	if devicePath == "" {
		devicePath = "/dev/" + info.DeviceIdentifier
	}
	rawPath := rawDevicePathFor(devicePath)

	return Disk{
		ID:          info.DeviceIdentifier,
		Path:        devicePath,
		RawPath:     rawPath,
		SizeBytes:   size,
		Internal:    info.Internal,
		WholeDisk:   info.WholeDisk,
		Physical:    strings.EqualFold(info.VirtualOrPhysical, "Physical"),
		Description: description,
	}, nil
}

// rawDevicePathFor turns "/dev/disk4" into "/dev/rdisk4". macOS exposes a
// raw character device per block device that bypasses the buffer cache
// and is ~5-10x faster for sequential dd writes.
func rawDevicePathFor(blockPath string) string {
	const prefix = "/dev/"
	if !strings.HasPrefix(blockPath, prefix) {
		return blockPath
	}
	name := strings.TrimPrefix(blockPath, prefix)
	if strings.HasPrefix(name, "r") {
		return blockPath
	}
	return prefix + "r" + name
}

// runDiskutil executes a diskutil subcommand and surfaces any error
// with stderr context.
func runDiskutil(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "diskutil", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("flash: diskutil %s: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// pipeDiskutilJSON runs `diskutil <args>` and pipes its plist output
// through `plutil -convert json -o - -`, returning the resulting JSON
// bytes. Both binaries ship with macOS so there's no external dep.
func pipeDiskutilJSON(ctx context.Context, args ...string) ([]byte, error) {
	diskutil := exec.CommandContext(ctx, "diskutil", args...)
	plutil := exec.CommandContext(ctx, "plutil", "-convert", "json", "-o", "-", "-")

	diskutilOut, err := diskutil.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("flash: diskutil stdout pipe: %w", err)
	}
	plutil.Stdin = diskutilOut

	var stdout, plutilErr, diskutilErr bytes.Buffer
	plutil.Stdout = &stdout
	plutil.Stderr = &plutilErr
	diskutil.Stderr = &diskutilErr

	if err := plutil.Start(); err != nil {
		return nil, fmt.Errorf("flash: start plutil: %w", err)
	}
	if err := diskutil.Run(); err != nil {
		// Wait for plutil to drain the pipe and exit before returning,
		// so we don't leak the child process.
		_ = plutil.Wait()
		return nil, fmt.Errorf("flash: diskutil %s: %w (stderr: %s)", strings.Join(args, " "), err, strings.TrimSpace(diskutilErr.String()))
	}
	if err := plutil.Wait(); err != nil {
		return nil, fmt.Errorf("flash: plutil convert: %w (stderr: %s)", err, strings.TrimSpace(plutilErr.String()))
	}
	if stdout.Len() == 0 {
		return nil, errors.New("flash: empty JSON from plutil")
	}
	return stdout.Bytes(), nil
}
