package vm

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func printInfo(out io.Writer, meta Metadata) {
	fmt.Fprintf(out, "id=%s\nname=%s\npreset=%s\ntarget=%s\nraw_xz=%s\nvm_dir=%s\nkairos_qcow2=%s\npersistent_qcow2=%s\nnetwork_mode=%s\nmac_address=%s\nssh_port=%d\napi_port=%d\nmonitor_port=%d\nqga_port=%d\nmemory_mb=%d\ncpus=%d\n",
		meta.ID, meta.Name, meta.Preset, meta.Target, meta.RawXZ, meta.VMDir, meta.KairosQCOW2, meta.PersistentQCOW2, meta.NetworkMode, macAddress(meta), meta.SSHPort, meta.APIPort, meta.MonitorPort, meta.QGAPort, meta.MemoryMB, meta.CPUs)
}

func printStatus(out io.Writer, meta Metadata) {
	state := "stopped"
	pid := 0
	if isRunning(meta) {
		state = "running"
		pid = readPID(meta)
	}
	fmt.Fprintf(out, "id=%s\nname=%s\nstate=%s\npid=%s\nnetwork_mode=%s\nmac_address=%s\nconsole_log=%s\nmonitor=127.0.0.1:%d\nqga=127.0.0.1:%d\n",
		meta.ID, meta.Name, state, formatPID(pid), meta.NetworkMode, macAddress(meta), meta.ConsoleLog, meta.MonitorPort, meta.QGAPort)
	if meta.SSHPort != 0 {
		fmt.Fprintf(out, "ssh=127.0.0.1:%d\n", meta.SSHPort)
	}
	if meta.APIPort != 0 {
		fmt.Fprintf(out, "api=https://127.0.0.1:%d\n", meta.APIPort)
	}
	if state == "running" {
		if ips, err := guestIPv4s(meta); err == nil && len(ips) > 0 {
			fmt.Fprintf(out, "guest_ipv4=%s\n", strings.Join(ips, ","))
		}
		if meta.SSHPort != 0 {
			fmt.Fprintf(out, "ssh_status=%s\n", portStatus(meta.SSHPort))
		}
		if meta.APIPort != 0 {
			fmt.Fprintf(out, "api_status=%s\n", portStatus(meta.APIPort))
		}
	}
}

func portStatus(port int) string {
	if localTCPPortOpen(port) {
		return "open"
	}
	return "closed"
}

func formatPID(value int) string {
	if value == 0 {
		return "unavailable"
	}
	return strconv.Itoa(value)
}
