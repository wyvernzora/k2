package vm

func (r Runner) Status(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	r.Reporter.KeyValues(statusFields(meta)...)
	return nil
}
