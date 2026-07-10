package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	sysDir   = "sys"
	classDir = "class"
	blockDir = "block"
)

var (
	sysRoot        = string(os.PathSeparator)
	devRoot        = "/dev"
	rescanWait     = time.Second
	currentTime    = time.Now
	sleepForRescan = time.Sleep
	statBlock      = os.Stat
)

func (m manager) targetDisk() (string, error) {
	if m.cfg.Disk != "auto" {
		if err := waitForBlock(m.cfg.Disk, m.cfg.WaitSeconds); err != nil {
			return "", err
		}
		resolved, err := filepath.EvalSymlinks(m.cfg.Disk)
		if err != nil {
			return "", fmt.Errorf("resolve target disk %s: %w", m.cfg.Disk, err)
		}
		disk, err := diskForDev(resolved, m.run)
		if err != nil {
			return "", fmt.Errorf("resolve target disk %s: %w", m.cfg.Disk, err)
		}
		boot, err := m.bootDisk()
		if err != nil {
			return "", fmt.Errorf("detect boot disk before using explicit target %s: %w", m.cfg.Disk, err)
		}
		if disk == boot {
			return "", fmt.Errorf("target disk %s resolves to boot disk %s", m.cfg.Disk, boot)
		}
		return disk, nil
	}

	bootDisk, _ := m.bootDisk()
	deadline := currentTime().Add(time.Duration(m.cfg.WaitSeconds) * time.Second)
	for {
		_ = m.run.Run("udevadm", "settle")
		disk, err := m.scanTargetDisk(bootDisk)
		if disk != "" || err != nil {
			return disk, err
		}
		if !currentTime().Before(deadline) {
			return "", nil
		}
		sleepForRescan(rescanWait)
	}
}

func (m manager) scanTargetDisk(bootDisk string) (string, error) {
	disks, err := filepath.Glob(sysBlockPath("*"))
	if err != nil {
		return "", err
	}
	for _, sysBlock := range disks {
		name := filepath.Base(sysBlock)
		if skipBlockName(name) {
			continue
		}
		disk := devPath(name)
		if _, err := os.Stat(disk); err != nil {
			continue
		}
		if bootDisk != "" && disk == bootDisk {
			continue
		}
		if m.diskHasMountedChild(disk) {
			m.log.Printf("skipping %s: it has mounted partitions", disk)
			continue
		}
		if m.diskHasAnyLabel(disk, []string{"COS_GRUB", "COS_OEM", "COS_RECOVERY", "COS_STATE"}) {
			continue
		}
		if m.diskHasPartitions(disk) && !m.diskHasLabel(disk, persistLabel) {
			m.log.Printf("skipping %s: it has partitions but no %s label", disk, persistLabel)
			continue
		}
		return disk, nil
	}
	return "", nil
}

func (m manager) bootDisk() (string, error) {
	for _, mountpoint := range []string{"/run/initramfs/cos-state", "/oem"} {
		src, _ := m.run.Output("findmnt", "-n", "-o", "SOURCE", mountpoint)
		if src == "" {
			continue
		}
		disk, err := diskForDev(src, m.run)
		if err == nil && disk != "" {
			m.log.Printf("detected boot disk %s from mounted %s", disk, mountpoint)
			return disk, nil
		}
	}
	for _, label := range []string{"COS_GRUB", "COS_OEM", "COS_RECOVERY", "COS_STATE"} {
		dev, _ := firstDeviceByLabel(label, m.run)
		if dev == "" {
			continue
		}
		disk, err := diskForDev(dev, m.run)
		if err == nil && disk != "" {
			m.log.Printf("detected boot disk %s from label %s", disk, label)
			return disk, nil
		}
	}
	return "", errors.New("boot disk not found")
}

func (m manager) diskHasLabel(disk, label string) bool {
	devices, _ := devicesByLabel(label, m.run)
	for _, dev := range devices {
		devDisk, err := diskForDev(dev, m.run)
		if err == nil && devDisk == disk {
			return true
		}
	}
	return false
}

func (m manager) diskHasAnyLabel(disk string, labels []string) bool {
	for _, label := range labels {
		if m.diskHasLabel(disk, label) {
			return true
		}
	}
	return false
}

func (m manager) diskHasMountedChild(disk string) bool {
	for _, child := range partitionChildren(disk) {
		dev := "/dev/" + filepath.Base(child)
		if out, _ := m.run.Output("findmnt", "-rn", "--source", dev); out != "" {
			return true
		}
	}
	return false
}

func (m manager) diskHasPartitions(disk string) bool {
	return len(partitionChildren(disk)) > 0
}

func skipBlockName(name string) bool {
	return strings.HasPrefix(name, "loop") ||
		strings.HasPrefix(name, "ram") ||
		strings.HasPrefix(name, "sr") ||
		strings.HasPrefix(name, "fd") ||
		strings.HasPrefix(name, "md") ||
		strings.HasPrefix(name, "dm-")
}

func FirstPartition(disk string) string {
	if disk != "" && disk[len(disk)-1] >= '0' && disk[len(disk)-1] <= '9' {
		return disk + "p1"
	}
	return disk + "1"
}

func waitForBlock(dev string, seconds int) error {
	for i := 0; i <= seconds; i++ {
		if st, err := statBlock(dev); err == nil && st.Mode()&os.ModeDevice != 0 {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("block device %s did not appear", dev)
}

func diskForDev(dev string, runner Runner) (string, error) {
	resolved := dev
	if out, err := runner.Output("readlink", "-f", dev); err == nil && out != "" {
		resolved = out
	}
	name := filepath.Base(resolved)
	sysPath, err := filepath.EvalSymlinks(sysClassBlockPath(name))
	if err == nil {
		if _, err := os.Stat(sysClassBlockPath(name, "partition")); err == nil {
			return "/dev/" + filepath.Base(filepath.Dir(sysPath)), nil
		}
	}
	if out, err := runner.Output("lsblk", "-no", "PKNAME", resolved); err == nil && out != "" {
		return "/dev/" + strings.Fields(out)[0], nil
	}
	return resolved, nil
}

func partitionNumber(dev string) (string, error) {
	out, err := os.ReadFile(sysClassBlockPath(filepath.Base(dev), "partition"))
	if err != nil {
		return "", fmt.Errorf("read partition number for %s: %w", dev, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func partitionChildren(disk string) []string {
	children, _ := filepath.Glob(sysBlockPath(filepath.Base(disk), "*"))
	var out []string
	for _, child := range children {
		if _, err := os.Stat(filepath.Join(child, "partition")); err == nil {
			out = append(out, child)
		}
	}
	return out
}

func sysClassBlockPath(parts ...string) string {
	return filepath.Join(append([]string{sysRoot, sysDir, classDir, blockDir}, parts...)...)
}

func sysBlockPath(parts ...string) string {
	return filepath.Join(append([]string{sysRoot, sysDir, blockDir}, parts...)...)
}

func devPath(name string) string {
	return filepath.Join(devRoot, name)
}
