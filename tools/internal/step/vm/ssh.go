package vm

import (
	"os/exec"
	"strconv"
)

func (r Runner) SSH(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	cmd := exec.Command("ssh", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null")
	if meta.SSHPort != 0 {
		cmd.Args = append(cmd.Args, "-p", strconv.Itoa(meta.SSHPort), "kairos@127.0.0.1")
	} else {
		ip, err := firstGuestIPv4(meta)
		if err != nil {
			return err
		}
		cmd.Args = append(cmd.Args, "kairos@"+ip)
	}
	return r.runInteractive(cmd)
}
