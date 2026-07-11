package vm

func (r Runner) Start(opts StartOptions) error {
	meta, err := loadMetadata(r.RepoRoot, opts.ID)
	if err != nil {
		return err
	}
	return r.start(meta, opts.Sudo)
}
