package remote

import (
	"slices"
	"testing"
)

func TestSSHArgsQuoteRemoteScriptAsSingleCommand(t *testing.T) {
	client := Client{
		Host: "127.0.0.1",
		Port: 2222,
		User: "kairos",
	}

	got := client.sshArgs("cat '/usr/share/k2/image-build/metadata.yaml'")
	want := []string{
		"-p",
		"2222",
		"-o",
		"StrictHostKeyChecking=no",
		"-o",
		"UserKnownHostsFile=/dev/null",
		"-o",
		"ConnectTimeout=10",
		"-o",
		"NumberOfPasswordPrompts=1",
		"kairos@127.0.0.1",
		"sh -lc 'cat '\"'\"'/usr/share/k2/image-build/metadata.yaml'\"'\"''",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("ssh args:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestSSHArgsUseAcceptNewHostKeyCheckingForNonLoopback(t *testing.T) {
	client := Client{
		Host: "10.10.9.10",
		Port: 22,
		User: "kairos",
	}

	got := client.sshArgs("true")
	if !slices.Contains(got, "StrictHostKeyChecking=accept-new") {
		t.Fatalf("ssh args missing accept-new host key mode: %#v", got)
	}
	if slices.Contains(got, "UserKnownHostsFile=/dev/null") {
		t.Fatalf("ssh args disabled known-hosts for non-loopback target: %#v", got)
	}
}

func TestSSHArgsDisableHostKeyCheckingForLocalhost(t *testing.T) {
	client := Client{
		Host: "localhost",
		Port: 2222,
		User: "kairos",
	}

	got := client.sshArgs("true")
	for _, want := range []string{
		"StrictHostKeyChecking=no",
		"UserKnownHostsFile=/dev/null",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("ssh args missing %q: %#v", want, got)
		}
	}
}

func TestSSHArgsIncludePasswordOptionsWhenPasswordAuthSelected(t *testing.T) {
	client := Client{
		Host:     "127.0.0.1",
		Port:     2222,
		User:     "kairos",
		authMode: authModePassword,
		password: "kairos",
	}

	got := client.sshArgs("true")
	for _, want := range []string{
		"PreferredAuthentications=password",
		"PubkeyAuthentication=no",
		"NumberOfPasswordPrompts=1",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("ssh args missing %q: %#v", want, got)
		}
	}
}

func TestSSHArgsIncludeBatchModeWhenLocalSSHSelected(t *testing.T) {
	client := Client{
		Host:     "k2-vbox",
		Port:     2222,
		User:     "kairos",
		authMode: authModeLocalSSH,
	}

	got := client.sshArgs("true")
	if !slices.Contains(got, "BatchMode=yes") {
		t.Fatalf("ssh args missing BatchMode option: %#v", got)
	}
	for _, want := range []string{
		"StrictHostKeyChecking=no",
		"UserKnownHostsFile=/dev/null",
	} {
		if !slices.Contains(got, want) {
			t.Fatalf("ssh args missing %q for local SSH config mode: %#v", want, got)
		}
	}
}
