package health

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWithHealthyNoPools(t *testing.T) {
	status := filepath.Join(t.TempDir(), "status")
	run := fakeRunner{
		outputs: map[string]string{
			"zpool list -H -o name": "",
		},
		runErrs: map[string]error{
			"systemctl is-failed --quiet rtslib-fb-targetctl.service": errors.New("not failed"),
		},
	}
	var stdout, stderr bytes.Buffer

	err := runWith(Config{StatusFile: status, SaveConfig: filepath.Join(t.TempDir(), "missing.json")}, run, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := "healthy: no ZFS pools imported\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertStatus(t, status, want)
}

func TestRunWithUnhealthyPoolAndFailedLIO(t *testing.T) {
	status := filepath.Join(t.TempDir(), "status")
	run := fakeRunner{
		outputs: map[string]string{
			"zpool list -H -o name":        "tank",
			"zpool list -H -o health tank": "DEGRADED",
		},
	}
	var stdout, stderr bytes.Buffer

	err := runWith(Config{StatusFile: status, SaveConfig: filepath.Join(t.TempDir(), "missing.json")}, run, &stdout, &stderr)
	if !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("err = %v, want ErrUnhealthy", err)
	}
	want := "UNHEALTHY: pool tank health DEGRADED; rtslib-fb-targetctl.service failed: LIO config not restored\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	assertStatus(t, status, want)
}

func TestRunWithPortalListening(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
		close(done)
	}()

	dir := t.TempDir()
	saveConfig := filepath.Join(dir, "saveconfig.json")
	status := filepath.Join(dir, "status")
	if err := os.WriteFile(saveConfig, []byte(`{"targets":[{},{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := fakeRunner{
		outputs: map[string]string{
			"zpool list -H -o name":        "tank",
			"zpool list -H -o health tank": "ONLINE",
		},
		runErrs: map[string]error{
			"systemctl is-failed --quiet rtslib-fb-targetctl.service": errors.New("not failed"),
		},
	}
	var stdout, stderr bytes.Buffer

	err = runWith(Config{SaveConfig: saveConfig, StatusFile: status, Portal: listener.Addr().String()}, run, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	want := "healthy: pool tank ONLINE; 2 iSCSI target(s), portal listening on " + portalPort(listener.Addr().String()) + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertStatus(t, status, want)
}

func assertStatus(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("status = %q, want %q", string(data), want)
	}
}

type fakeRunner struct {
	outputs map[string]string
	outErrs map[string]error
	runErrs map[string]error
}

func (r fakeRunner) Run(name string, args ...string) error {
	key := name + " " + strings.Join(args, " ")
	if err, ok := r.runErrs[key]; ok {
		return err
	}
	return nil
}

func (r fakeRunner) Output(name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	return r.outputs[key], r.outErrs[key]
}
