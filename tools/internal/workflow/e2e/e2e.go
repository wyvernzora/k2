package e2e

func e2eRegistration() Registration {
	return Registration{Name: "e2e", Help: "Run end-to-end validation harnesses.", Order: 20, Command: &e2eCmd{}}
}

type e2eCmd struct {
	List    e2eListCmd    `cmd:"" help:"List available e2e scenarios."`
	Run     e2eRunCmd     `cmd:"" help:"Run an e2e scenario."`
	Storage e2eStorageCmd `cmd:"" help:"Alias for: e2e run storage-pvc."`
}
