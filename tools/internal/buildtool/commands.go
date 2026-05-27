package buildtool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type commandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
	Name     string
	Args     []string
	Elapsed  time.Duration
}

func (r commandResult) Error(prefix string) error {
	if r.Err == nil {
		return nil
	}
	message := fmt.Sprintf("%s %s failed after %s: %v\n%s", r.Name, strings.Join(r.Args, " "), r.Elapsed.Round(time.Millisecond), r.Err, string(r.Stderr))
	if prefix != "" {
		return fmt.Errorf("%s: %s", prefix, message)
	}
	return errors.New(message)
}

func runCommand(ctx context.Context, dir string, env []string, name string, args ...string) commandResult {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			exitCode = exit.ExitCode()
		}
	}
	return commandResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: exitCode,
		Err:      err,
		Name:     name,
		Args:     args,
		Elapsed:  time.Since(start),
	}
}

func runCapture(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return runCaptureEnv(ctx, dir, nil, name, args...)
}

func runCaptureEnv(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	result := runCommand(ctx, dir, env, name, args...)
	if result.Err != nil {
		return nil, result.Error("")
	}
	return result.Stdout, nil
}
