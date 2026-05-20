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

func resolvePort(spec string) (int, error) {
	if spec == "" {
		return 0, fmt.Errorf("empty port spec")
	}
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
		return findFreePort(start, end)
	}
	port, err := strconv.Atoi(spec)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: %w", spec, err)
	}
	return port, nil
}

func resolveOptionalPort(networkMode string, override string, presetSpec string) (int, error) {
	spec := firstNonEmpty(override, presetSpec)
	if spec == "" {
		if networkMode == "" || networkMode == "user" {
			return 0, fmt.Errorf("user-mode networking requires SSH and API host forwards")
		}
		return 0, nil
	}
	return resolvePort(spec)
}

func findFreePort(start int, end int) (int, error) {
	for port := start; port <= end; port++ {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			_ = listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port found in %d-%d", start, end)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
