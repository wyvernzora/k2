package vm

func (r Runner) Info(id string) error {
	meta, err := loadMetadata(r.RepoRoot, id)
	if err != nil {
		return err
	}
	r.Reporter.KeyValues(infoFields(meta)...)
	return nil
}
