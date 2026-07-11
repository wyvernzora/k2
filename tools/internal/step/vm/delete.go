package vm

import (
	"fmt"
	"os"
)

func (r Runner) Delete(opts DeleteOptions) error {
	metas, err := r.resolveTargetVMs(opts.ID, opts.All)
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		r.logf("no VMs found")
		return nil
	}
	if !opts.Force {
		if err := r.confirmDelete(metas); err != nil {
			return err
		}
	}

	for _, meta := range metas {
		if err := r.stop(meta); err != nil {
			return fmt.Errorf("stop %s before delete: %w", meta.ID, err)
		}
		if err := os.RemoveAll(meta.VMDir); err != nil {
			return fmt.Errorf("delete %s: %w", meta.ID, err)
		}
		r.successf("deleted %s", meta.ID)
	}
	return nil
}
