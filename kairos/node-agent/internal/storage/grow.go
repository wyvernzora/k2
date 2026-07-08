package storage

func (m manager) growOriginal() error {
	dev, _ := firstDeviceByLabel(persistLabel, m.run)
	if dev == "" {
		m.log.Printf("no %s filesystem found yet; nothing to grow", persistLabel)
		return nil
	}
	return m.grow(dev)
}

func (m manager) grow(dev string) error {
	fstype, _ := m.run.Output("blkid", "-s", "TYPE", "-o", "value", dev)
	if !isExtFilesystem(fstype) {
		m.log.Printf("%s has unsupported filesystem %q; skipping grow", dev, fstype)
		return nil
	}
	if mounted, _ := m.run.Output("findmnt", "-rn", "--source", dev); mounted != "" {
		m.log.Printf("%s is already mounted; skipping offline grow", dev)
		return nil
	}
	disk, err := diskForDev(dev, m.run)
	if err != nil {
		return err
	}
	partNum, err := partitionNumber(dev)
	if err != nil {
		return err
	}
	m.log.Printf("growing %s to the end of %s", dev, disk)
	_ = m.run.Run("parted", "-s", disk, "resizepart", partNum, "100%")
	_ = m.run.Run("partprobe", disk)
	_ = m.run.Run("udevadm", "settle")
	_ = m.run.Run("resize2fs", dev)
	return nil
}

func isExtFilesystem(fstype string) bool {
	switch fstype {
	case "ext2", "ext3", "ext4":
		return true
	default:
		return false
	}
}
