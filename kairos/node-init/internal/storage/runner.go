package storage

import (
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(name string, args ...string) error
	Output(name string, args ...string) (string, error)
}

type OSRunner struct{}

func (OSRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (OSRunner) Output(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func ensureCommands(commands ...string) error {
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			return err
		}
	}
	return nil
}
