package flash

import (
	"errors"
)

// Size bands used to classify the disks that appear after rpiboot. The
// CM4 Lite eMMC is 32 GB nominal (≈31.9 GiB raw). The ComputeBlade
// carrier exposes a 256 GB NVMe through the same USB-C surface. These
// ranges are generous on both ends so that future variants (16/64 GB
// eMMC, 128/512 GB NVMe) still classify cleanly.
const (
	emmcMinBytes uint64 = 14 * 1024 * 1024 * 1024   // 14 GiB
	emmcMaxBytes uint64 = 72 * 1024 * 1024 * 1024   // 72 GiB
	nvmeMinBytes uint64 = 120 * 1024 * 1024 * 1024  // 120 GiB
	nvmeMaxBytes uint64 = 1100 * 1024 * 1024 * 1024 // ~1 TiB
)

// ErrAmbiguousDisks is returned when classify can't tell which of two
// same-band disks is the intended target. The operator should re-run
// with explicit --emmc / --nvme.
var ErrAmbiguousDisks = errors.New("flash: more than one disk matches a size band; supply --emmc / --nvme explicitly")

// DiffDisks returns the disks present in `after` but not in `before`
// (matched by Disk.ID). Stable order: same as `after`.
func DiffDisks(before, after []Disk) []Disk {
	seen := make(map[string]struct{}, len(before))
	for _, d := range before {
		seen[d.ID] = struct{}{}
	}
	out := make([]Disk, 0, len(after))
	for _, d := range after {
		if _, ok := seen[d.ID]; ok {
			continue
		}
		out = append(out, d)
	}
	return out
}

// Classification holds the result of bucketing a set of disks into
// "this is the eMMC" and "this is the NVMe". Either may be zero-valued
// if no candidate fell into the corresponding band; the runner decides
// whether that's fatal based on whether --zero-nvme was requested.
type Classification struct {
	EMMC    Disk
	HasEMMC bool
	NVMe    Disk
	HasNVMe bool
}

// Classify buckets disks into eMMC- and NVMe-sized candidates. Returns
// ErrAmbiguousDisks when more than one disk falls into the same band.
// Disks that don't fit either band are ignored — the runner surfaces a
// friendly error when no eMMC candidate ends up identified.
