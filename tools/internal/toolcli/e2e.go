package toolcli

type e2eCmd struct {
	List    e2eListCmd    `cmd:"" help:"List available e2e scenarios."`
	Run     e2eRunCmd     `cmd:"" help:"Run an e2e scenario."`
	Storage e2eStorageCmd `cmd:"" help:"Alias for: e2e run storage-pvc."`
}
