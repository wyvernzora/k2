package vm

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (r Runner) start(meta Metadata, sudo bool) error {
	if isRunning(meta) {
		r.successf("%s already running%s", meta.Name, pidSuffix(meta))
		return nil
	}
	firmware, err := qemuFirmware()
	if err != nil {
		return err
	}
	if err := os.WriteFile(meta.ConsoleLog, nil, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(meta.QEMULog, nil, 0o644); err != nil {
		return err
	}
	_ = os.Remove(meta.PIDFile)
	// QEMU's `-chardev socket,server=on` does not unlink an existing
	// socket on bind, so a stale file from a previous run would make
	// it fail with EADDRINUSE. Remove it preemptively.
	if meta.ConsoleSocket != "" {
		_ = os.Remove(meta.ConsoleSocket)
	}

	netdev, err := netdevArg(meta)
	if err != nil {
		return err
	}
	args := qemuArgs(meta, firmware, netdev)
	r.logf("starting %s", meta.Name)
	cmd := exec.Command("qemu-system-aarch64", args...)
	if sudo {
		cmd = exec.Command("sudo", append([]string{"qemu-system-aarch64"}, args...)...)
		cmd.Stdin = r.stdin()
	}
	if err := runCommand(cmd); err != nil {
		return qemuStartError(meta, err, sudo)
	}
	if err := r.ensureConsoleSocketReady(meta, sudo); err != nil {
		r.logf("warning: console may be unusable: %v", err)
	}
	r.successf("started %s%s", meta.Name, pidSuffix(meta))
	r.logf("ssh: k2-tools vm ssh %s", meta.ID)
	if meta.APIPort != 0 {
		r.logf("api: https://127.0.0.1:%d", meta.APIPort)
	}
	r.logf("console: k2-tools vm console %s", meta.ID)
	return nil
}

// ensureConsoleSocketReady waits up to a few seconds for QEMU's
// chardev socket to appear on disk, then — when QEMU was launched via
// sudo — relaxes its mode to 0660 so the local operator (in group
// `staff` on macOS) can connect. Connecting to a unix-domain socket
// needs write permission on the file; without this fixup the operator
// gets EACCES against a root:staff 0750 socket for every vmnet-shared
// VM. The chmod runs through sudo so it works regardless of who owns
// the socket; sudo creds are still cached from the QEMU launch a
// moment ago, so no extra prompt.
func (r Runner) ensureConsoleSocketReady(meta Metadata, sudo bool) error {
	if meta.ConsoleSocket == "" {
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(meta.ConsoleSocket); err == nil {
			if !sudo {
				return nil
			}
			return runCommand(exec.Command("sudo", "chmod", "0660", meta.ConsoleSocket))
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("console socket %s did not appear within 5s", meta.ConsoleSocket)
}

func qemuArgs(meta Metadata, firmware string, netdev string) []string {
	args := []string{
		"-name", meta.Name,
		"-machine", "virt,accel=hvf",
		"-cpu", "host",
		"-smp", strconv.Itoa(meta.CPUs),
		"-m", strconv.Itoa(meta.MemoryMB),
		"-bios", firmware,
		"-drive", "if=none,file=" + meta.KairosQCOW2 + ",format=qcow2,id=system",
		"-device", "virtio-blk-pci,drive=system,bootindex=0",
		"-netdev", netdev,
		"-device", "virtio-net-pci,netdev=net0,mac=" + macAddress(meta),
		"-device", "virtio-serial-pci",
		"-chardev", fmt.Sprintf("socket,host=127.0.0.1,port=%d,server=on,wait=off,id=qga0", meta.QGAPort),
		"-device", "virtserialport,chardev=qga0,name=org.qemu.guest_agent.0",
		"-monitor", fmt.Sprintf("tcp:127.0.0.1:%d,server=on,wait=off", meta.MonitorPort),
		// Bidirectional serial: QEMU listens on a unix-domain socket,
		// `k2-tools vm console` dials in. logfile= keeps the persistent
		// log so tailing `console.log` directly still works for passive
		// observation when nobody is attached.
		"-chardev", fmt.Sprintf("socket,id=console0,path=%s,server=on,wait=off,logfile=%s,logappend=on", meta.ConsoleSocket, meta.ConsoleLog),
		"-serial", "chardev:console0",
		"-display", "none",
		"-pidfile", meta.PIDFile,
		"-D", meta.QEMULog,
		"-daemonize",
	}
	if meta.PersistentQCOW2 != "" {
		if _, err := os.Stat(meta.PersistentQCOW2); err == nil {
			args = append(args, "-drive", "if=none,file="+meta.PersistentQCOW2+",format=qcow2,id=persistent", "-device", "virtio-blk-pci,drive=persistent")
		}
	}
	for _, disk := range meta.ExtraDisks {
		if _, err := os.Stat(disk.QCOW2); err == nil {
			args = append(args, "-drive", "if=none,file="+disk.QCOW2+",format=qcow2,id="+disk.ID, "-device", "virtio-blk-pci,drive="+disk.ID)
		}
	}
	return args
}

func (r Runner) stop(meta Metadata) error {
	if !isRunning(meta) {
		r.logf("%s is not running", meta.Name)
		_ = os.Remove(meta.PIDFile)
		return nil
	}
	pid := readPID(meta)
	if err := writeMonitorCommand(meta, "system_powerdown\n"); err == nil {
		for range 20 {
			if !pidRunning(pid) {
				_ = os.Remove(meta.PIDFile)
				r.successf("stopped %s", meta.Name)
				return nil
			}
			time.Sleep(time.Second)
		}
	}
	if pid == 0 {
		return fmt.Errorf("powerdown did not complete and PID file is unreadable; stop sudo-launched QEMU manually")
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	_ = os.Remove(meta.PIDFile)
	return nil
}

func netdevArg(meta Metadata) (string, error) {
	switch meta.NetworkMode {
	case "", "user":
		if meta.SSHPort == 0 || meta.APIPort == 0 {
			return "", fmt.Errorf("user-mode networking requires SSH and API host forwards")
		}
		return fmt.Sprintf("user,id=net0,hostfwd=tcp:127.0.0.1:%d-:22,hostfwd=tcp:127.0.0.1:%d-:6443", meta.SSHPort, meta.APIPort), nil
	case "vmnet-shared":
		return "vmnet-shared,id=net0", nil
	default:
		return "", fmt.Errorf("unsupported VM network mode %q", meta.NetworkMode)
	}
}

func qemuStartError(meta Metadata, err error, sudo bool) error {
	detail := strings.TrimSpace(readSmallFile(meta.QEMULog))
	if detail != "" {
		return fmt.Errorf("%w\nQEMU log %s:\n%s", err, meta.QEMULog, detail)
	}
	if meta.NetworkMode == "vmnet-shared" && !sudo {
		return fmt.Errorf("%w\nQEMU vmnet-shared requires macOS vmnet privileges. Try: k2-tools vm start --sudo %s", err, meta.ID)
	}
	return err
}

func qemuFirmware() (string, error) {
	candidates := []string{
		"/opt/homebrew/share/qemu/edk2-aarch64-code.fd",
		"/usr/local/share/qemu/edk2-aarch64-code.fd",
		"/usr/share/qemu-efi-aarch64/QEMU_EFI.fd",
		"/usr/share/AAVMF/AAVMF_CODE.fd",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find AArch64 QEMU firmware")
}

func macAddress(meta Metadata) string {
	if meta.MACAddress != "" {
		return meta.MACAddress
	}
	return deterministicMACAddress(meta.ID)
}

func deterministicMACAddress(id string) string {
	sum := sha256.Sum256([]byte("k2-tools-vm:" + id))
	return fmt.Sprintf("52:4b:32:%02x:%02x:%02x", sum[0], sum[1], sum[2])
}

func writeMonitorCommand(meta Metadata, command string) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(meta.MonitorPort)), time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write([]byte(command))
	return err
}

func readSmallFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	const max = 4096
	if len(data) > max {
		data = data[len(data)-max:]
	}
	return string(data)
}

func runCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func checkCreateCommands() error {
	for _, name := range []string{"qemu-img", "qemu-system-aarch64", "xz"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("missing %s: %w", name, err)
		}
	}
	return nil
}

func isRunning(meta Metadata) bool {
	pid := readPID(meta)
	if pid != 0 {
		return pidRunning(pid)
	}
	return localTCPPortOpen(meta.MonitorPort) || localTCPPortOpen(meta.QGAPort)
}

func readPID(meta Metadata) int {
	data, err := os.ReadFile(meta.PIDFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func pidSuffix(meta Metadata) string {
	pid := readPID(meta)
	if pid == 0 {
		return " (pid unavailable)"
	}
	return fmt.Sprintf(" with pid %d", pid)
}

func pidRunning(pid int) bool {
	if pid == 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func localTCPPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
