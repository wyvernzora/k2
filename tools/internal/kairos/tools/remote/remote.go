package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
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
	// IdentityFile optionally points at an unencrypted private key used
	// for key auth, for operators whose agents cannot load the operator
	// key (or headless runs with no agent at all).
	IdentityFile string
	// InsecureHostKey skips known_hosts pinning. Local test VMs recycle
	// addresses (loopback ports, vmnet leases) across generations with
	// fresh host keys, so pinning guarantees spurious mismatches.
	InsecureHostKey bool
	// NoPasswordPrompt fails auth instead of prompting on stdin. Set by
	// non-interactive callers (e2e harness) where a prompt would render
	// invisibly under the progress UI and block Ctrl-C.
	NoPasswordPrompt bool

	authMode authMode
	password string
}

func (c Client) Address() string {
	return c.User + "@" + c.Host
}

// SwapIO redirects this Client's Stdout, Stderr, and Logger to write
// into `w`. Returns a func that restores the previous IO. The
// canonical use is inside a ui.Workflow Shell step, where the
// bubbletea-managed io.Writer owns the terminal region for the
// duration:
//
//	wf.Shell("Upload bundle", func(ctx context.Context, sh ui.Step) error {
//	    defer client.SwapIO(sh)()
//	    return client.UploadDir(localDir)
//	})
//
// Without the redirection, the Client's Stdout/Stderr writes to
// os.Stdout/os.Stderr race with bubbletea's terminal management and
// corrupt the spinner/scrollback display.
func (c *Client) SwapIO(w io.Writer) func() {
	prevStdout, prevStderr, prevLogger := c.Stdout, c.Stderr, c.Logger
	c.Stdout = w
	c.Stderr = w
	c.Logger = func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(line, "\n") {
			line += "\n"
		}
		_, _ = w.Write([]byte(line))
	}
	return func() {
		c.Stdout = prevStdout
		c.Stderr = prevStderr
		c.Logger = prevLogger
	}
}

func (c *Client) EnsureAuth() error {
	if c.authMode != authModeUnknown {
		return nil
	}

	// With an explicit identity, key auth goes first: hardened nodes reject
	// passwords anyway, and modern sshd (PerSourcePenalties, OpenSSH 9.8+)
	// penalty-boxes sources that rack up failed auth attempts.
	if c.IdentityFile != "" {
		c.logf("probing SSH auth with identity file and local SSH agent")
		if err := c.probeLocalSSH(); err == nil {
			c.authMode = authModeLocalSSH
			c.logf("using local SSH key auth")
			return nil
		} else {
			c.logf("local SSH key auth did not succeed: %v", err)
		}
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

	if c.IdentityFile == "" {
		c.logf("probing SSH auth with local SSH agent and keys")
		if err := c.probeLocalSSH(); err == nil {
			c.authMode = authModeLocalSSH
			c.logf("using local SSH agent/key auth")
			return nil
		} else {
			c.logf("local SSH agent/key auth did not succeed: %v", err)
		}
	}

	if c.NoPasswordPrompt {
		return fmt.Errorf("SSH authentication to %s failed (key and default password probes exhausted; interactive prompt disabled)", c.Address())
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
	return c.WaitForAuthCtx(context.Background(), timeout)
}

func (c *Client) WaitForAuthCtx(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		c.ResetAuth()
		passwordErr := c.probePassword("kairos")
		if passwordErr == nil {
			c.authMode = authModePassword
			c.password = "kairos"
			c.logf("using default kairos password auth")
			return nil
		}

		localErr := c.probeLocalSSH()
		if localErr == nil {
			c.authMode = authModeLocalSSH
			c.logf("using local SSH agent/key auth")
			return nil
		}
		lastErr = errors.Join(passwordErr, localErr)

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for SSH auth to %s: %w", c.Address(), lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) ReadFile(file string) ([]byte, error) {
	if err := c.EnsureAuth(); err != nil {
		return nil, err
	}
	c.logf("ssh read %s:%s", c.Address(), file)
	out, err := c.runCapture("cat " + shellQuote(file))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ReadSudoFile(file string) ([]byte, error) {
	if err := c.EnsureAuth(); err != nil {
		return nil, err
	}
	c.logf("ssh sudo read %s:%s", c.Address(), file)
	out, err := c.runCapture("sudo cat " + shellQuote(file))
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
	remoteDirBytes, err := c.runCapture("mktemp -d /tmp/k2-tools.XXXXXX")
	if err != nil {
		return "", err
	}
	remoteDir := strings.TrimSpace(string(remoteDirBytes))
	if remoteDir == "" {
		return "", fmt.Errorf("remote mktemp returned empty path")
	}
	c.logf("sftp upload %s -> %s:%s", localDir, c.Address(), remoteDir)
	if err := c.sftpUploadDir(localDir, remoteDir); err != nil {
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

// RunAllowDisconnect forgives a dropped connection ONLY once the command
// actually started: reboot scripts sever the session mid-run by design. A
// disconnect-looking error at dial/handshake time (reset, EOF) means the
// script never ran at all and must fail loudly — swallowing it reported
// "install complete" against untouched nodes.
func (c *Client) RunAllowDisconnect(script string) error {
	err := c.Run(script)
	if err == nil {
		return nil
	}
	var runErr sessionRunError
	if errors.As(err, &runErr) && isSSHDisconnect(err) {
		return nil
	}
	return err
}

// sessionRunError marks failures raised by the remote command itself,
// after dial and session setup succeeded.
type sessionRunError struct{ err error }

func (e sessionRunError) Error() string { return e.err.Error() }
func (e sessionRunError) Unwrap() error { return e.err }

func (c *Client) run(script string) error {
	return c.runWithWriters(script, writer(c.Stdout), writer(c.Stderr))
}

func (c *Client) runCapture(script string) ([]byte, error) {
	var out bytes.Buffer
	err := c.runWithWriters(script, &out, writer(c.Stderr))
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (c *Client) runWithWriters(script string, stdout io.Writer, stderr io.Writer) error {
	client, err := c.dialSelectedAuth()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("open SSH session to %s: %w", c.Address(), err)
	}
	defer session.Close()

	session.Stdout = stdout
	session.Stderr = stderr
	if err := session.Run("sh -lc " + shellQuote(script)); err != nil {
		return sessionRunError{fmt.Errorf("ssh %s failed: %w", c.Address(), err)}
	}
	return nil
}

func (c *Client) sftpUploadDir(localDir string, remoteDir string) error {
	client, err := c.dialSelectedAuth()
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("open SFTP session to %s: %w", c.Address(), err)
	}
	defer sftpClient.Close()

	return filepath.WalkDir(localDir, func(localPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(localDir, localPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		remotePath := path.Join(remoteDir, filepath.ToSlash(rel))
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return sftpClient.MkdirAll(remotePath)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := sftpClient.MkdirAll(path.Dir(remotePath)); err != nil {
			return err
		}
		return uploadFile(sftpClient, localPath, remotePath, info.Mode().Perm())
	})
}

func uploadFile(client *sftp.Client, localPath string, remotePath string, mode fs.FileMode) error {
	local, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer local.Close()

	remote, err := client.OpenFile(remotePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer remote.Close()

	if _, err := io.Copy(remote, local); err != nil {
		return err
	}
	return client.Chmod(remotePath, mode)
}

func (c *Client) probePassword(password string) error {
	client, err := c.dial(passwordAuthMethods(password))
	if err != nil {
		return fmt.Errorf("ssh password probe failed: %w", err)
	}
	client.Close()
	return nil
}

func (c *Client) probeLocalSSH() error {
	var auth []ssh.AuthMethod
	if c.IdentityFile != "" {
		method, err := identityFileAuthMethod(c.IdentityFile)
		if err != nil {
			return err
		}
		auth = append(auth, method)
	}
	agentAuth, err := localSSHAuthMethods()
	if err == nil {
		auth = append(auth, agentAuth...)
	} else if len(auth) == 0 {
		return err
	}
	client, err := c.dial(auth)
	if err != nil {
		return fmt.Errorf("ssh local key probe failed: %w", err)
	}
	client.Close()
	return nil
}

func (c *Client) dialSelectedAuth() (*ssh.Client, error) {
	switch c.authMode {
	case authModePassword:
		return c.dial(passwordAuthMethods(c.password))
	case authModeLocalSSH:
		var auth []ssh.AuthMethod
		if c.IdentityFile != "" {
			method, err := identityFileAuthMethod(c.IdentityFile)
			if err != nil {
				return nil, err
			}
			auth = append(auth, method)
		}
		agentAuth, err := localSSHAuthMethods()
		if err == nil {
			auth = append(auth, agentAuth...)
		} else if len(auth) == 0 {
			return nil, err
		}
		return c.dial(auth)
	default:
		return nil, fmt.Errorf("SSH auth has not been selected")
	}
}

func (c *Client) dial(auth []ssh.AuthMethod) (*ssh.Client, error) {
	hostKeyCallback, err := c.hostKeyCallback()
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            c.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", c.networkAddress(), config)
}

func passwordAuthMethods(password string) []ssh.AuthMethod {
	return []ssh.AuthMethod{
		ssh.Password(password),
		ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = password
			}
			return answers, nil
		}),
	}
}

func identityFileAuthMethod(path string) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read identity file %s: %w", path, err)
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse identity file %s (must be unencrypted): %w", path, err)
	}
	return ssh.PublicKeys(signer), nil
}

func localSSHAuthMethods() ([]ssh.AuthMethod, error) {
	var auth []ssh.AuthMethod
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			auth = append(auth, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}

	signers := defaultPrivateKeySigners()
	if len(signers) > 0 {
		auth = append(auth, ssh.PublicKeys(signers...))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("no SSH agent or unencrypted default private keys are available")
	}
	return auth, nil
}

func defaultPrivateKeySigners() []ssh.Signer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var signers []ssh.Signer
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		data, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}
	return signers
}

func (c *Client) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if c.InsecureHostKey || isLoopbackHost(c.Host) {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	knownHostsFile, err := knownHostsPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(knownHostsFile), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(knownHostsFile, os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("prepare known_hosts file: %w", err)
	}
	file.Close()

	callback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := callback(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
			if appendErr := appendKnownHost(knownHostsFile, knownHostTarget(c.Host, c.Port), key); appendErr != nil {
				return appendErr
			}
			c.logf("accepted new SSH host key for %s", c.Host)
			return nil
		}
		return err
	}, nil
}

func knownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

func appendKnownHost(file string, target string, key ssh.PublicKey) error {
	handle, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer handle.Close()
	_, err = fmt.Fprintln(handle, knownhosts.Line([]string{target}, key))
	return err
}

func knownHostTarget(host string, port int) string {
	if port == 22 {
		return host
	}
	return knownhosts.Normalize(net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port)))
}

func (c *Client) networkAddress() string {
	return net.JoinHostPort(strings.Trim(c.Host, "[]"), strconv.Itoa(c.Port))
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

// logf is the remote client's narration channel. Callers are expected
// to wire `Logger` to ui.Reporter (or another sink) at construction
// time. When unset we silently discard rather than emit a raw
// `k2-tools: ...` line: that would bypass the Reporter's plain-mode
// discipline (the Reporter is the single source of `k2-tools:`
// prefixed output across the binary).
func (c *Client) logf(format string, args ...any) {
	if c.Logger != nil {
		c.Logger(format, args...)
	}
}

func isSSHDisconnect(err error) bool {
	if err == nil {
		return false
	}
	var missing *ssh.ExitMissingError
	if errors.As(err, &missing) {
		return true
	}
	message := err.Error()
	for _, part := range []string{
		"EOF",
		"connection lost",
		"connection reset",
		"use of closed network connection",
		"broken pipe",
	} {
		if strings.Contains(message, part) {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
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
