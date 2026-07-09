package vm

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
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
	if sudo {
		// QEMU under sudo writes a root-owned 0600 pidfile; without this
		// fixup readPID silently returns 0 for the VM's whole lifetime and
		// stop/delete orphan the root QEMU process.
		chmod := exec.Command("sudo", "chmod", "0644", meta.PIDFile)
		chmod.Stdin = r.stdin()
		_ = chmod.Run()
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
	if err := writeMonitorCommand(meta, "system_powerdown\n"); err == nil {
		if r.waitStopped(meta, 20*time.Second) {
			return nil
		}
	}
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
		for _, pid := range qemuPIDs(meta) {
			r.killPID(pid, sig)
		}
		if r.waitStopped(meta, 10*time.Second) {
			return nil
		}
	}
	return fmt.Errorf("%s is still running after powerdown/TERM/KILL (pids %v); stop it manually", meta.Name, qemuPIDs(meta))
}

// waitStopped polls until no QEMU process for this VM remains. Death must
// be confirmed by process/port evidence — an unreadable pidfile is NOT
// evidence of death (sudo-launched QEMU writes a root-0600 pidfile), and
// declaring success early is how VM dirs got deleted under live QEMUs,
// leaving orphaned processes squatting on the VM's deterministic MAC.
func (r Runner) waitStopped(meta Metadata, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !isRunning(meta) {
			_ = os.Remove(meta.PIDFile)
			r.successf("stopped %s", meta.Name)
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Second)
	}
}

func (r Runner) killPID(pid int, sig syscall.Signal) {
	err := syscall.Kill(pid, sig)
	if err == nil || errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return
	}
	// EPERM: the QEMU was launched via sudo and runs as root.
	kill := exec.Command("sudo", "kill", fmt.Sprintf("-%d", int(sig)), strconv.Itoa(pid))
	kill.Stdin = r.stdin()
	_ = kill.Run()
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

// runCommand captures subprocess output and surfaces it only on failure:
// these helpers run underneath the ui progress renderer, and raw writes to
// the tty corrupt its layout.
func runCommand(cmd *exec.Cmd) error {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", cmd.Args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
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
	if len(qemuPIDs(meta)) > 0 {
		return true
	}
	return localTCPPortOpen(meta.MonitorPort) || localTCPPortOpen(meta.QGAPort)
}

// qemuPIDs returns every live QEMU process belonging to this VM: the
// pidfile pid when readable, plus a pgrep sweep over the unique
// `-name <meta.Name>` argument. The sweep is what finds root QEMUs whose
// 0600 pidfile the operator cannot read, and duplicate instances leaked
// by earlier stop/delete bugs.
func qemuPIDs(meta Metadata) []int {
	seen := map[int]bool{}
	var pids []int
	if pid := readPID(meta); pid != 0 && pidRunning(pid) {
		seen[pid] = true
		pids = append(pids, pid)
	}
	// Pattern deliberately omits the leading dash of `-name` so pgrep does
	// not parse it as a flag; the boundary group keeps e.g. worker-1 from
	// matching worker-10.
	out, err := exec.Command("pgrep", "-f", "name "+regexp.QuoteMeta(meta.Name)+"( |$)").Output()
	if err != nil {
		return pids
	}
	for _, field := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(field)
		if err != nil || seen[pid] || pid == os.Getpid() {
			continue
		}
		seen[pid] = true
		pids = append(pids, pid)
	}
	return pids
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
