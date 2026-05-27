package remote

import (
	"errors"
	"io"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestLoopbackHostDetection(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "[127.0.0.1]", "::1", "[::1]"} {
		if !isLoopbackHost(host) {
			t.Fatalf("expected %q to be loopback", host)
		}
	}
	if isLoopbackHost("10.10.9.10") {
		t.Fatalf("non-loopback host detected as loopback")
	}
}

func TestKnownHostTargetUsesBracketedPortForNonDefaultPort(t *testing.T) {
	got := knownHostTarget("10.10.9.10", 2222)
	want := "[10.10.9.10]:2222"
	if got != want {
		t.Fatalf("known host target = %q, want %q", got, want)
	}
}

func TestKnownHostTargetUsesPlainHostForDefaultPort(t *testing.T) {
	got := knownHostTarget("10.10.9.10", 22)
	want := "10.10.9.10"
	if got != want {
		t.Fatalf("known host target = %q, want %q", got, want)
	}
}

func TestRunAllowDisconnectAcceptsSSHDisconnect(t *testing.T) {
	if !isSSHDisconnect(io.EOF) {
		t.Fatalf("EOF should be treated as SSH disconnect")
	}
	var missing *ssh.ExitMissingError
	if !isSSHDisconnect(missing) {
		t.Fatalf("missing exit status should be treated as SSH disconnect")
	}
	if isSSHDisconnect(errors.New("remote command failed")) {
		t.Fatalf("ordinary error should not be treated as SSH disconnect")
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	got := shellQuote("cat '/tmp/file'")
	want := "'cat '\"'\"'/tmp/file'\"'\"''"
	if got != want {
		t.Fatalf("shellQuote = %q, want %q", got, want)
	}
}
