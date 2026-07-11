package storage

const (
	persistLabel = "COS_PERSISTENT"
	newLabel     = "COS_PERSIST_NEW"
	oldLabel     = "COS_PERSIST_OLD"
)

type Config struct {
	Disk        string
	Marker      string
	LogPrefix   string
	WaitSeconds int
	VerifyOnly  bool
}

func Normalize(cfg Config) Config {
	if cfg.Disk == "" {
		cfg.Disk = "auto"
	}
	if cfg.Marker == "" {
		cfg.Marker = ".k2-persistent-ok"
	}
	if cfg.LogPrefix == "" {
		cfg.LogPrefix = "kairos-persistent"
	}
	if cfg.WaitSeconds <= 0 {
		cfg.WaitSeconds = 30
	}
	return cfg
}
