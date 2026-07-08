package storage

import "github.com/wyvernzora/k2/kairos/node-agent/internal/runner"

type Runner = runner.Runner

type OSRunner = runner.OSRunner

func ensureCommands(commands ...string) error {
	return runner.EnsureCommands(commands...)
}
