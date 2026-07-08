package storage

import (
	"fmt"
	"os"
)

type manager struct {
	cfg Config
	run Runner
	log logger
}

func Run(cfg Config) error {
	return runWith(cfg, OSRunner{})
}

func runWith(cfg Config, runner Runner) error {
	cfg = Normalize(cfg)
	os.Setenv("PATH", "/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:"+os.Getenv("PATH"))

	log := newLogger(cfg.LogPrefix)
	defer log.Close()

	m := manager{cfg: cfg, run: runner, log: log}
	if cfg.VerifyOnly {
		return m.verify()
	}
	return m.prepare()
}

func (m manager) prepare() error {
	if err := ensureCommands("blkid", "e2label", "findmnt", "lsblk", "mkfs.ext4", "mount", "parted", "partprobe", "readlink", "resize2fs", "tar", "udevadm", "umount", "wipefs"); err != nil {
		return fmt.Errorf("missing required command: %w", err)
	}

	_ = m.run.Run("udevadm", "settle")

	targetDisk, err := m.targetDisk()
	if err != nil {
		if m.cfg.Required || m.cfg.Disk != "auto" {
			return err
		}
		m.log.Printf("%v; keeping original %s", err, persistLabel)
		return m.growOriginal()
	}
	if targetDisk == "" {
		if m.cfg.Required {
			return fmt.Errorf("no target disk found")
		}
		m.log.Printf("no second disk found; keeping original %s", persistLabel)
		return m.growOriginal()
	}

	return m.prepareTargetDisk(targetDisk)
}

func (m manager) prepareTargetDisk(targetDisk string) error {
	targetPart := FirstPartition(targetDisk)
	m.log.Printf("using %s for %s", targetPart, persistLabel)

	existing, err := m.persistentOnDisk(targetDisk)
	if err != nil {
		return err
	}
	if existing != "" {
		m.log.Printf("target disk already has %s at %s", persistLabel, existing)
		if err := m.relabelOtherPersistent(targetDisk); err != nil {
			return err
		}
		return m.grow(existing)
	}

	m.log.Printf("initializing %s as dedicated persistent disk", targetDisk)
	_ = m.run.Run("wipefs", "-a", targetDisk)
	if err := m.run.Run("parted", "-s", targetDisk, "mklabel", "gpt"); err != nil {
		return fmt.Errorf("create GPT on %s: %w", targetDisk, err)
	}
	if err := m.run.Run("parted", "-s", "-a", "optimal", targetDisk, "mkpart", "kairos-persistent", "ext4", "1MiB", "100%"); err != nil {
		return fmt.Errorf("create persistent partition on %s: %w", targetDisk, err)
	}
	_ = m.run.Run("partprobe", targetDisk)
	_ = m.run.Run("udevadm", "settle")

	if err := waitForBlock(targetPart, m.cfg.WaitSeconds); err != nil {
		return err
	}
	if err := m.run.Run("mkfs.ext4", "-F", "-L", newLabel, targetPart); err != nil {
		return fmt.Errorf("format %s: %w", targetPart, err)
	}

	current, err := firstDeviceByLabel(persistLabel, m.run)
	if err != nil {
		return err
	}
	if err := m.copyPersistent(current, targetPart); err != nil {
		return err
	}
	if err := m.labelExt(targetPart, persistLabel); err != nil {
		return err
	}
	if err := m.relabelOtherPersistent(targetDisk); err != nil {
		return err
	}
	if err := m.grow(targetPart); err != nil {
		return err
	}
	m.log.Printf("persistent setup complete")
	return nil
}
