package flash

import "fmt"

func Classify(disks []Disk) (Classification, error) {
	var result Classification
	for _, d := range disks {
		switch {
		case d.SizeBytes >= emmcMinBytes && d.SizeBytes <= emmcMaxBytes:
			if result.HasEMMC {
				return Classification{}, fmt.Errorf("%w: %s and %s both look like eMMC", ErrAmbiguousDisks, result.EMMC.Path, d.Path)
			}
			result.EMMC = d
			result.HasEMMC = true
		case d.SizeBytes >= nvmeMinBytes && d.SizeBytes <= nvmeMaxBytes:
			if result.HasNVMe {
				return Classification{}, fmt.Errorf("%w: %s and %s both look like NVMe", ErrAmbiguousDisks, result.NVMe.Path, d.Path)
			}
			result.NVMe = d
			result.HasNVMe = true
		}
	}
	return result, nil
}
