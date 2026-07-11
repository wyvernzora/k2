package vm

import (
	"os/exec"
	"strconv"
)

func (r Runner) Monitor(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	return r.runInteractive(exec.Command("nc", "127.0.0.1", strconv.Itoa(meta.MonitorPort)))
}
