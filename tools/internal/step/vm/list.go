package vm

func (r Runner) List() error {
	metas, err := listMetadata(r.RepoRoot)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(metas))
	for _, meta := range metas {
		state := "stopped"
		if isRunning(meta) {
			state = "running"
		}
		rows = append(rows, []string{meta.ID, meta.Preset, state, listIP(meta), meta.VMDir})
	}
	r.Reporter.Table([]string{"ID", "PRESET", "STATE", "IP", "DIR"}, rows)
	return nil
}
