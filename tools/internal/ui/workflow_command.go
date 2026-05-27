package ui

import (
	"context"
	"os"
	"os/exec"
)

// CommandSpec describes a subprocess for Command, Attach, and JobGroup
// command jobs.
type CommandSpec struct {
	Name  string
	Args  []string
	Dir   string
	Env   []string
	Stdin *os.File
}

func (s CommandSpec) command(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.Name, s.Args...)
	cmd.Dir = s.Dir
	if len(s.Env) > 0 {
		cmd.Env = append(os.Environ(), s.Env...)
	}
	return cmd
}

func (w *Workflow) Command(label string, spec CommandSpec) *StepHandle {
	return w.Shell(label, func(ctx context.Context, sh Step) error {
		cmd := spec.command(ctx)
		cmd.Stdout = sh
		cmd.Stderr = sh
		cmd.Stdin = spec.Stdin
		return cmd.Run()
	})
}

func (w *Workflow) Attach(label string, spec CommandSpec) *StepHandle {
	return w.add("attach "+label, func(ctx context.Context) error {
		w.reporter.Infof("Attaching: %s", label)
		cmd := spec.command(ctx)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			w.reporter.Errorf("%s failed: %v", label, err)
			return err
		}
		w.reporter.Successf("%s finished", label)
		return nil
	})
}
