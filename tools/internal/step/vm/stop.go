package vm

import "fmt"

func (r Runner) Stop(opts StopOptions) error {
	metas, err := r.resolveTargetVMs(opts.ID, opts.All)
	if err != nil {
		return err
	}
	if len(metas) == 0 {
		r.logf("no VMs found")
		return nil
	}
	for _, meta := range metas {
		if err := r.stop(meta); err != nil {
			return fmt.Errorf("stop %s: %w", meta.ID, err)
		}
	}
	return nil
}
