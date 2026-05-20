package remote

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

type authMode int

const (
	authModeUnknown authMode = iota
	authModePassword
	authModeLocalSSH
)

type Client struct {
	Host   string
	Port   int
	User   string
	Stdout io.Writer
	Stderr io.Writer
	Logger func(format string, args ...any)

	authMode authMode
	password string
}

func (c Client) Address() string {
	return c.User + "@" + c.Host
}

func (c *Client) EnsureAuth() error {
	if c.authMode != authModeUnknown {
		return nil
	}

	c.logf("probing SSH auth with default kairos password")
	if err := c.probePassword("kairos"); err == nil {
		c.authMode = authModePassword
		c.password = "kairos"
		c.logf("using default kairos password auth")
		return nil
	} else {
		c.logf("default kairos password auth did not succeed: %v", err)
	}

	c.logf("probing SSH auth with local SSH config")
	if err := c.probeLocalSSH(); err == nil {
		c.authMode = authModeLocalSSH
		c.logf("using local SSH config auth")
		return nil
	} else {
		c.logf("local SSH config auth did not succeed: %v", err)
	}

	password, err := c.promptPassword()
	if err != nil {
		return err
	}
	if err := c.probePassword(password); err != nil {
		return fmt.Errorf("password auth failed for %s: %w", c.Address(), err)
	}
	c.authMode = authModePassword
	c.password = password
	c.logf("using prompted password auth")
	return nil
}

func (c *Client) ResetAuth() {
	c.authMode = authModeUnknown
	c.password = ""
}

func (c *Client) WaitForAuth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		c.ResetAuth()
		if err := c.probePassword("kairos"); err == nil {
			c.authMode = authModePassword
			c.password = "kairos"
			c.logf("using default kairos password auth")
			return nil
		} else {
			lastErr = err
		}
		if err := c.probeLocalSSH(); err == nil {
			c.authMode = authModeLocalSSH
			c.logf("using local SSH config auth")
			return nil
		} else {
			lastErr = err
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for SSH auth to %s: %w", c.Address(), lastErr)
		}
		time.Sleep(5 * time.Second)
	}
}

func (c *Client) ReadFile(path string) ([]byte, error) {
	if err := c.EnsureAuth(); err != nil {
		return nil, err
	}
	c.logf("ssh read %s:%s", c.Address(), path)
	out, err := c.runCapture("cat " + shellQuote(path))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ReadSudoFile(path string) ([]byte, error) {
	if err := c.EnsureAuth(); err != nil {
		return nil, err
	}
	c.logf("ssh sudo read %s:%s", c.Address(), path)
	out, err := c.runCapture("sudo cat " + shellQuote(path))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Check(script string) error {
	if err := c.EnsureAuth(); err != nil {
		return err
	}
	_, err := c.runCapture(script)
	return err
}

func (c *Client) Capture(script string) ([]byte, error) {
	if err := c.EnsureAuth(); err != nil {
		return nil, err
	}
	return c.runCapture(script)
}

func (c *Client) UploadDir(localDir string) (string, error) {
	if err := c.EnsureAuth(); err != nil {
		return "", err
	}
	c.logf("ssh create remote temp dir on %s", c.Address())
	remoteDirBytes, err := c.runCapture("mktemp -d /tmp/k2-provision.XXXXXX")
	if err != nil {
		return "", err
	}
	remoteDir := strings.TrimSpace(string(remoteDirBytes))
	if remoteDir == "" {
		return "", fmt.Errorf("remote mktemp returned empty path")
	}
	c.logf("scp upload %s -> %s:%s", localDir, c.Address(), remoteDir)
	if err := c.scp(localDir+string(filepath.Separator)+".", remoteDir+"/"); err != nil {
		return "", err
	}
	return remoteDir, nil
}

func (c *Client) Run(script string) error {
	if err := c.EnsureAuth(); err != nil {
		return err
	}
	c.logf("ssh run install script on %s", c.Address())
	return c.run(script)
}

func (c *Client) RunAllowDisconnect(script string) error {
	err := c.Run(script)
	if err == nil || isSSHDisconnectExit(err) {
		return nil
	}
	return err
}

func (c *Client) run(script string) error {
	cmd := exec.Command("ssh", c.sshArgs(script)...)
	cmd.Stdout = writer(c.Stdout)
	cmd.Stderr = writer(c.Stderr)
	cleanup, err := c.configureCommandAuth(cmd)
	if err != nil {
		return err
	}
	defer cleanup()
	return cmd.Run()
}

func (c *Client) runCapture(script string) ([]byte, error) {
	cmd := exec.Command("ssh", c.sshArgs(script)...)
	cmd.Stderr = writer(c.Stderr)
	cleanup, err := c.configureCommandAuth(cmd)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ssh %s failed: %w", c.Address(), err)
	}
	return out, nil
}

func (c *Client) probePassword(password string) error {
	cmd := exec.Command("ssh", c.sshArgsWithOptions("true", passwordSSHOptions(), false)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = writer(c.Stderr)
	cleanup, err := withPasswordAskpass(cmd, password)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh password probe failed: %w", err)
	}
	return nil
}

func (c *Client) probeLocalSSH() error {
	cmd := exec.Command("ssh", c.sshArgsWithOptions("true", localSSHOptions(), true)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = writer(c.Stderr)
	cmd.Stdin = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh local config probe failed: %w", err)
	}
	return nil
}

func (c *Client) promptPassword() (string, error) {
	fmt.Fprintf(writer(c.Stderr), "SSH password for %s: ", c.Address())
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(writer(c.Stderr))
	if err != nil {
		return "", fmt.Errorf("read SSH password: %w", err)
	}
	if len(password) == 0 {
		return "", fmt.Errorf("SSH password is empty")
	}
	return string(password), nil
}

func (c *Client) logf(format string, args ...any) {
	if c.Logger != nil {
		c.Logger(format, args...)
		return
	}
	fmt.Fprintf(writer(c.Stderr), "k2-provision: "+format+"\n", args...)
}

func (c *Client) scp(local string, remote string) error {
	args := []string{
		"-P", strconv.Itoa(c.Port),
	}
	args = append(args, c.authOptions()...)
	args = append(args, c.hostKeyOptions(c.authMode == authModeLocalSSH)...)
	args = append(args,
		"-o", "ConnectTimeout=10",
		"-o", "NumberOfPasswordPrompts=1",
		"-r",
		local,
		c.Address()+":"+remote,
	)
	cmd := exec.Command("scp", args...)
	cmd.Stdout = writer(c.Stdout)
	cmd.Stderr = writer(c.Stderr)
	cleanup, err := c.configureCommandAuth(cmd)
	if err != nil {
		return err
	}
	defer cleanup()
	return cmd.Run()
}

func (c *Client) sshArgs(script string) []string {
	return c.sshArgsWithOptions(script, c.authOptions(), c.authMode == authModeLocalSSH)
}

func (c *Client) sshArgsWithOptions(script string, options []string, localSSHConfig bool) []string {
	args := []string{
		"-p", strconv.Itoa(c.Port),
	}
	args = append(args, options...)
	args = append(args, c.hostKeyOptions(localSSHConfig)...)
	args = append(args,
		"-o", "ConnectTimeout=10",
		"-o", "NumberOfPasswordPrompts=1",
		c.Address(),
		"sh -lc "+shellQuote(script),
	)
	return args
}

func (c *Client) hostKeyOptions(localSSHConfig bool) []string {
	if localSSHConfig || isLoopbackHost(c.Host) {
		return []string{
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
		}
	}
	return []string{
		"-o", "StrictHostKeyChecking=accept-new",
	}
}

func isSSHDisconnectExit(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.ExitStatus() == 255
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost":
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func (c *Client) authOptions() []string {
	switch c.authMode {
	case authModePassword:
		return passwordSSHOptions()
	case authModeLocalSSH:
		return localSSHOptions()
	default:
		return nil
	}
}

func (c *Client) configureCommandAuth(cmd *exec.Cmd) (func(), error) {
	switch c.authMode {
	case authModePassword:
		return withPasswordAskpass(cmd, c.password)
	case authModeLocalSSH:
		cmd.Stdin = nil
		return func() {}, nil
	default:
		cmd.Stdin = os.Stdin
		return func() {}, nil
	}
}

func passwordSSHOptions() []string {
	return []string{
		"-o", "PreferredAuthentications=password",
		"-o", "PubkeyAuthentication=no",
	}
}

func localSSHOptions() []string {
	return []string{
		"-o", "BatchMode=yes",
	}
}

func withPasswordAskpass(cmd *exec.Cmd, password string) (func(), error) {
	dir, err := os.MkdirTemp("", "k2-provision-askpass-*")
	if err != nil {
		return nil, fmt.Errorf("create SSH askpass helper: %w", err)
	}
	path := filepath.Join(dir, "askpass")
	script := "#!/bin/sh\nprintf '%s\\n' " + shellQuote(password) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("write SSH askpass helper: %w", err)
	}
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(),
		"SSH_ASKPASS="+path,
		"SSH_ASKPASS_REQUIRE=force",
		"DISPLAY=k2-provision",
	)
	return func() {
		_ = os.RemoveAll(dir)
	}, nil
}

func writer(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
