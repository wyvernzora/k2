package storage

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func (m manager) persistentOnDisk(disk string) (string, error) {
	devices, err := devicesByLabel(persistLabel, m.run)
	if err != nil {
		return "", err
	}
	for _, dev := range devices {
		devDisk, err := diskForDev(dev, m.run)
		if err == nil && devDisk == disk {
			return dev, nil
		}
	}
	return "", nil
}

func (m manager) relabelOtherPersistent(targetDisk string) error {
	devices, err := devicesByLabel(persistLabel, m.run)
	if err != nil {
		return err
	}
	for _, dev := range devices {
		devDisk, err := diskForDev(dev, m.run)
		if err != nil {
			return fmt.Errorf("resolve persistent device %s: %w", dev, err)
		}
		if devDisk == targetDisk {
			continue
		}
		if err := m.labelExt(dev, m.cfg.OldLabel); err != nil {
			return err
		}
	}
	return nil
}

func (m manager) labelExt(dev, label string) error {
	fstype, _ := m.run.Output("blkid", "-s", "TYPE", "-o", "value", dev)
	if !isExtFilesystem(fstype) {
		return fmt.Errorf("%s has unsupported filesystem %q; cannot label", dev, fstype)
	}
	m.log.Printf("labeling %s as %s", dev, label)
	return m.run.Run("e2label", dev, label)
}

func firstDeviceByLabel(label string, runner Runner) (string, error) {
	devices, err := devicesByLabel(label, runner)
	if err != nil || len(devices) == 0 {
		return "", err
	}
	return devices[0], nil
}

func devicesByLabel(label string, runner Runner) ([]string, error) {
	out, err := runner.Output("blkid", "-o", "device", "-t", "LABEL="+label)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return nil, nil
		}
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Fields(out), nil
}
