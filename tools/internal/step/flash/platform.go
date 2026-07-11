package flash

import (
	"context"
	"errors"
)

// Disk is a platform-agnostic description of one storage device the
// flasher might touch. macOS populates this via `diskutil`; a future
// Linux implementation would populate it via `lsblk` / sysfs.
type Disk struct {
	// ID is the platform-specific short identifier, e.g. "disk4" on
	// macOS or "sda" on Linux. Used to disambiguate disks within one
	// run; should not be assumed stable across reboots.
	ID string
	// Path is the block device path, e.g. "/dev/disk4".
	Path string
	// RawPath is the path the flasher passes to `dd of=` for writes.
	// On macOS this is the raw character device (`/dev/rdiskN`) which
	// is dramatically faster than the buffered block device.
	RawPath string
	// SizeBytes is the total capacity. 0 if unknown.
	SizeBytes uint64
	// Internal is true if the platform reports the disk as an internal
	// drive (built-in SSD, etc.); the flasher refuses to touch these.
	Internal bool
	// WholeDisk is true if the device is a whole disk rather than a
	// partition slice. The flasher refuses anything that isn't.
	WholeDisk bool
	// Physical is true if the device is a physical disk rather than a
	// virtual one (synthesized, disk image, etc.).
	Physical bool
	// Description is a human-readable summary suitable for inclusion in
	// confirmation prompts (e.g. "Apple SDXC Reader Media, 31.9 GB").
	Description string
}

// Platform abstracts the OS-specific disk operations the flasher needs.
// One implementation is built per GOOS via build tags. Today only
// darwin is implemented; everything else returns ErrUnsupportedPlatform
// from CurrentPlatform().
type Platform interface {
	// EnumerateExternalDisks returns every external, whole, physical
	// disk the platform can currently see. Order is unspecified.
	EnumerateExternalDisks(ctx context.Context) ([]Disk, error)
	// DiskByID returns the disk with the given platform-specific ID,
	// or an error if the disk is not present or not eligible for flashing
	// (not external / not whole / not physical / etc.).
	DiskByID(ctx context.Context, id string) (Disk, error)
	// Unmount tears down any mounted filesystems on the given disk so
	// the flasher can safely `dd` to it.
	Unmount(ctx context.Context, d Disk) error
	// Eject releases the disk so the operator can disconnect it without
	// "improperly removed" warnings.
	Eject(ctx context.Context, d Disk) error
}

// ErrUnsupportedPlatform is returned by CurrentPlatform on hosts where
// no `flash` implementation exists yet.
var ErrUnsupportedPlatform = errors.New("flash: this platform is not supported (macOS only today; contributions welcome)")

// CurrentPlatform returns the Platform implementation for the host OS,
// or ErrUnsupportedPlatform if none exists. The actual constructor is
// supplied by a build-tagged file (platform_darwin.go for macOS;
// platform_other.go everywhere else).
func CurrentPlatform() (Platform, error) {
	return currentPlatform()
}
