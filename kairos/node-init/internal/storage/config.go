package storage

import (
	"fmt"
	"strings"
)

const (
	persistLabel = "COS_PERSISTENT"
	newLabel     = "COS_PERSIST_NEW"
)

type Mode string

const (
	ModeOptional Mode = "optional"
	ModeRequired Mode = "required"
)

type Config struct {
	Disk         string
	Required     bool
	OldLabel     string
	Marker       string
	LogPrefix    string
	WaitSeconds  int
	VerifyPrefix string
	VerifyOnly   bool
}

func ParseMode(mode string) (Mode, error) {
	switch Mode(strings.ToLower(mode)) {
	case ModeOptional:
		return ModeOptional, nil
	case ModeRequired:
		return ModeRequired, nil
	default:
		return "", fmt.Errorf("unknown mode %q", mode)
	}
}

func (m Mode) Required() bool {
	return m == ModeRequired
}

func Normalize(cfg Config) Config {
	if cfg.Disk == "" {
		cfg.Disk = "auto"
	}
	if cfg.OldLabel == "" {
		cfg.OldLabel = "COS_PERSIST_OLD"
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
