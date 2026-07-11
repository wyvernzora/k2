package vm

func (r Runner) Presets() error {
	presets, err := listPresets(r.RepoRoot)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(presets))
	for _, preset := range presets {
		rows = append(rows, []string{preset.name, preset.Description})
	}
	r.Reporter.Table([]string{"PRESET", "DESCRIPTION"}, rows)
	return nil
}
