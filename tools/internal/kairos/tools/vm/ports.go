package vm

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func resolveVMID(value string) (string, error) {
	if value != "" {
		return normalizeID(value), nil
	}
	generated, err := randomID()
	if err != nil {
		return "", err
	}
	return normalizeID(generated), nil
}

func randomID() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func forwardPortSpec(preset Preset, name string, fallback string) string {
	for _, forward := range preset.Forwards {
		if forward.Name == name {
			return forward.HostPort
		}
	}
	return fallback
}

func resolvePort(spec string, taken map[int]bool) (int, error) {
	if spec == "" {
		return 0, fmt.Errorf("empty port spec")
	}
	// `auto:START-END` is the preset-file form meaning "find a free
	// port in this range." The range-search behavior below already
	// implements that, so strip the prefix and fall through.
	spec = strings.TrimPrefix(spec, "auto:")
	if strings.Contains(spec, "-") {
		parts := strings.SplitN(spec, "-", 2)
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid port range %q", spec)
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid port range %q: %w", spec, err)
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid port range %q: %w", spec, err)
		}
		return findFreePort(start, end, taken)
	}
	port, err := strconv.Atoi(spec)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: %w", spec, err)
	}
	return port, nil
}

func resolveOptionalPort(networkMode string, override string, presetSpec string, taken map[int]bool) (int, error) {
	spec := firstNonEmpty(override, presetSpec)
	if spec == "" {
		if networkMode == "" || networkMode == "user" {
			return 0, fmt.Errorf("user-mode networking requires SSH and API host forwards")
		}
		return 0, nil
	}
	return resolvePort(spec, taken)
}

// findFreePort skips ports recorded in other VMs' metadata even when they
// are not currently bound: a stopped VM's monitor/QGA ports are free at the
// OS level, and handing them to a new VM would make the stopped VM's port
// identity ambiguous forever after.
func findFreePort(start int, end int, taken map[int]bool) (int, error) {
	for port := start; port <= end; port++ {
		if taken[port] {
			continue
		}
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found in %d-%d", start, end)
}

// recordedPorts collects every port claimed by existing VM metadata.
func recordedPorts(repoRoot string) (map[int]bool, error) {
	metas, err := listMetadata(repoRoot)
	if err != nil {
		return nil, err
	}
	taken := map[int]bool{}
	for _, meta := range metas {
		for _, port := range []int{meta.SSHPort, meta.APIPort, meta.MonitorPort, meta.QGAPort} {
			if port != 0 {
				taken[port] = true
			}
		}
	}
	return taken, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
