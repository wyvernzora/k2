package health

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	oldDial := dialPortal
	dialPortal = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return fakeConn{}, nil
	}
	t.Cleanup(func() { dialPortal = oldDial })

	dir := t.TempDir()
	saveConfig := filepath.Join(dir, "saveconfig.json")
	status := filepath.Join(dir, "status")
	if err := os.WriteFile(saveConfig, []byte(`{"targets":[{},{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := fakeRunner{
		outputs: map[string]string{
			"zpool list -H -o name":              "tank",
			"zpool list -H -o health tank":       "ONLINE",
			"zfs get -H -o value keystatus tank": "available",
		},
		runErrs: map[string]error{
			"systemctl is-failed --quiet rtslib-fb-targetctl.service": errors.New("not failed"),
		},
	}
	var stdout, stderr bytes.Buffer

	portal := "127.0.0.1:3260"
	err := runWith(Config{SaveConfig: saveConfig, StatusFile: status, Portal: portal}, run, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := "healthy: pool tank ONLINE; pool tank keys loaded; 2 iSCSI target(s), portal listening on " + portalPort(portal) + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertStatus(t, status, want)
}

func TestRunWithPoolKeyStatuses(t *testing.T) {
	tests := []struct {
		name       string
		keyStatus  string
		keyErr     error
		wantErr    bool
		wantOutput string
	}{
		{
			name:       "available",
			keyStatus:  "available",
			wantOutput: "healthy: pool tank ONLINE; pool tank keys loaded\n",
		},
		{
			name:       "none",
			keyStatus:  "none",
			wantOutput: "healthy: pool tank ONLINE; pool tank unencrypted\n",
		},
		{
			name:       "dash",
			keyStatus:  "-",
			wantOutput: "healthy: pool tank ONLINE; pool tank unencrypted\n",
		},
		{
			name:       "unavailable",
			keyStatus:  "unavailable",
			wantErr:    true,
			wantOutput: "UNHEALTHY: pool tank ONLINE; pool tank keys unavailable\n",
		},
		{
			name:       "error",
			keyErr:     errors.New("zfs failed"),
			wantErr:    true,
			wantOutput: "UNHEALTHY: pool tank ONLINE; pool tank keystatus check failed: zfs failed\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := filepath.Join(t.TempDir(), "status")
			run := fakeRunner{
				outputs: map[string]string{
					"zpool list -H -o name":              "tank",
					"zpool list -H -o health tank":       "ONLINE",
					"zfs get -H -o value keystatus tank": tt.keyStatus,
				},
				outErrs: map[string]error{
					"zfs get -H -o value keystatus tank": tt.keyErr,
				},
				runErrs: map[string]error{
					"systemctl is-failed --quiet rtslib-fb-targetctl.service": errors.New("not failed"),
				},
			}
			var stdout, stderr bytes.Buffer

			err := runWith(Config{StatusFile: status, SaveConfig: filepath.Join(t.TempDir(), "missing.json")}, run, &stdout, &stderr)
			if tt.wantErr {
				if !errors.Is(err, ErrUnhealthy) {
					t.Fatalf("err = %v, want ErrUnhealthy", err)
				}
				if stderr.String() != tt.wantOutput {
					t.Fatalf("stderr = %q, want %q", stderr.String(), tt.wantOutput)
				}
				if stdout.Len() != 0 {
					t.Fatalf("stdout = %q, want empty", stdout.String())
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if stdout.String() != tt.wantOutput {
					t.Fatalf("stdout = %q, want %q", stdout.String(), tt.wantOutput)
				}
				if stderr.Len() != 0 {
					t.Fatalf("stderr = %q, want empty", stderr.String())
				}
			}
			assertStatus(t, status, tt.wantOutput)
		})
	}
}

func TestRunWithPortalNotListeningIsUnhealthy(t *testing.T) {
	oldDial := dialPortal
	dialPortal = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, errors.New("connect: connection refused")
	}
	t.Cleanup(func() { dialPortal = oldDial })

	dir := t.TempDir()
	saveConfig := filepath.Join(dir, "saveconfig.json")
	status := filepath.Join(dir, "status")
	if err := os.WriteFile(saveConfig, []byte(`{"targets":[{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := fakeRunner{
		outputs: map[string]string{
			"zpool list -H -o name": "",
		},
		runErrs: map[string]error{
			"systemctl is-failed --quiet rtslib-fb-targetctl.service": errors.New("not failed"),
		},
	}
	var stdout, stderr bytes.Buffer

	addr := "127.0.0.1:3261"
	err := runWith(Config{SaveConfig: saveConfig, StatusFile: status, Portal: addr}, run, &stdout, &stderr)
	if !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("err = %v, want ErrUnhealthy", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	got := stderr.String()
	if !strings.HasPrefix(got, "UNHEALTHY: ") || !strings.Contains(got, "portal "+addr+" not listening") {
		t.Fatalf("stderr = %q, want not listening unhealthy note", got)
	}
	assertStatus(t, status, got)
}

func TestRunWithUnparseableSaveConfigIsUnhealthy(t *testing.T) {
	dir := t.TempDir()
	saveConfig := filepath.Join(dir, "saveconfig.json")
	status := filepath.Join(dir, "status")
	if err := os.WriteFile(saveConfig, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := fakeRunner{
		outputs: map[string]string{
			"zpool list -H -o name": "",
		},
		runErrs: map[string]error{
			"systemctl is-failed --quiet rtslib-fb-targetctl.service": errors.New("not failed"),
		},
	}
	var stdout, stderr bytes.Buffer

	err := runWith(Config{SaveConfig: saveConfig, StatusFile: status}, run, &stdout, &stderr)
	if !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("err = %v, want ErrUnhealthy", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "saveconfig.json unparseable") {
		t.Fatalf("stderr = %q, want unparseable note", got)
	}
	assertStatus(t, status, got)
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

type fakeConn struct{}

func (fakeConn) Read([]byte) (int, error)         { return 0, nil }
func (fakeConn) Write([]byte) (int, error)        { return 0, nil }
func (fakeConn) Close() error                     { return nil }
func (fakeConn) LocalAddr() net.Addr              { return fakeAddr("local") }
func (fakeConn) RemoteAddr() net.Addr             { return fakeAddr("remote") }
func (fakeConn) SetDeadline(time.Time) error      { return nil }
func (fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (fakeConn) SetWriteDeadline(time.Time) error { return nil }

type fakeAddr string

func (a fakeAddr) Network() string { return string(a) }
func (a fakeAddr) String() string  { return string(a) }

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
