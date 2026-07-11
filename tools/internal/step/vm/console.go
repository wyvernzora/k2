package vm

func (r Runner) Console(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	return r.attachConsole(meta)
}
