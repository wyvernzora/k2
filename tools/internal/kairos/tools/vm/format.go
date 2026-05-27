package vm

import (
	"io"
	"strconv"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/ui"
)

// infoFields builds the canonical Info display: every persistent piece
// of metadata Kairos VMs carry, in stable order. Returned as []ui.KV so
// the Reporter can render aligned key/value pairs.
func infoFields(meta Metadata) []ui.KV {
	return []ui.KV{
		{Key: "id", Value: meta.ID},
		{Key: "name", Value: meta.Name},
		{Key: "preset", Value: meta.Preset},
		{Key: "target", Value: meta.Target},
		{Key: "raw_xz", Value: meta.RawXZ},
		{Key: "vm_dir", Value: meta.VMDir},
		{Key: "kairos_qcow2", Value: meta.KairosQCOW2},
		{Key: "persistent_qcow2", Value: meta.PersistentQCOW2},
		{Key: "network_mode", Value: meta.NetworkMode},
		{Key: "mac_address", Value: macAddress(meta)},
		{Key: "ssh_port", Value: strconv.Itoa(meta.SSHPort)},
		{Key: "api_port", Value: strconv.Itoa(meta.APIPort)},
		{Key: "monitor_port", Value: strconv.Itoa(meta.MonitorPort)},
		{Key: "qga_port", Value: strconv.Itoa(meta.QGAPort)},
		{Key: "memory_mb", Value: strconv.Itoa(meta.MemoryMB)},
		{Key: "cpus", Value: strconv.Itoa(meta.CPUs)},
	}
}

// statusFields builds the Status display. Same shape as infoFields but
// only the runtime-relevant subset, plus live state (PID, ssh/api
// reachability, guest IPv4).
func statusFields(meta Metadata) []ui.KV {
	state := "stopped"
	pid := 0
	if isRunning(meta) {
		state = "running"
		pid = readPID(meta)
	}
	fields := []ui.KV{
		{Key: "id", Value: meta.ID},
		{Key: "name", Value: meta.Name},
		{Key: "state", Value: state},
		{Key: "pid", Value: formatPID(pid)},
		{Key: "network_mode", Value: meta.NetworkMode},
		{Key: "mac_address", Value: macAddress(meta)},
		{Key: "console_log", Value: meta.ConsoleLog},
		{Key: "monitor", Value: "127.0.0.1:" + strconv.Itoa(meta.MonitorPort)},
		{Key: "qga", Value: "127.0.0.1:" + strconv.Itoa(meta.QGAPort)},
	}
	if meta.SSHPort != 0 {
		fields = append(fields, ui.KV{Key: "ssh", Value: "127.0.0.1:" + strconv.Itoa(meta.SSHPort)})
	}
	if meta.APIPort != 0 {
		fields = append(fields, ui.KV{Key: "api", Value: "https://127.0.0.1:" + strconv.Itoa(meta.APIPort)})
	}
	if state == "running" {
		if ips, err := guestIPv4s(meta); err == nil && len(ips) > 0 {
			fields = append(fields, ui.KV{Key: "guest_ipv4", Value: strings.Join(ips, ",")})
		}
		if meta.SSHPort != 0 {
			fields = append(fields, ui.KV{Key: "ssh_status", Value: portStatus(meta.SSHPort)})
		}
		if meta.APIPort != 0 {
			fields = append(fields, ui.KV{Key: "api_status", Value: portStatus(meta.APIPort)})
		}
	}
	return fields
}

// printInfo / printStatus retained for back-compat with any caller
// that still passes an io.Writer (unused now but cheap to keep).
// New code should call reporter.KeyValues(infoFields(meta)...) directly.
func printInfo(out io.Writer, meta Metadata) {
	for _, kv := range infoFields(meta) {
		_, _ = io.WriteString(out, kv.Key+"="+kv.Value+"\n")
	}
}

func printStatus(out io.Writer, meta Metadata) {
	for _, kv := range statusFields(meta) {
		_, _ = io.WriteString(out, kv.Key+"="+kv.Value+"\n")
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
