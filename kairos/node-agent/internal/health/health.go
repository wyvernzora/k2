package health

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wyvernzora/k2/kairos/node-agent/internal/runner"
)

const (
	defaultSaveConfig = "/etc/rtslib-fb-target/saveconfig.json"
	defaultStatusFile = "/run/k2-storage-health/status"
	defaultPortal     = "127.0.0.1:3260"
)

var ErrUnhealthy = errors.New("storage unhealthy")

type Config struct {
	SaveConfig string
	StatusFile string
	Portal     string
}

func Run(cfg Config) error {
	return runWith(cfg, runner.OSRunner{}, os.Stdout, os.Stderr)
}

func runWith(cfg Config, run runner.Runner, stdout, stderr io.Writer) error {
	cfg = normalize(cfg)

	var notes []string
	unhealthy := false
	fail := func(note string) {
		unhealthy = true
		notes = append(notes, note)
	}

	pools, err := run.Output("zpool", "list", "-H", "-o", "name")
	if err != nil && pools == "" {
		fail(fmt.Sprintf("zpool list failed: %v", err))
	} else {
		names := strings.Fields(pools)
		if len(names) == 0 {
			notes = append(notes, "no ZFS pools imported")
		}
		for _, pool := range names {
			health, err := run.Output("zpool", "list", "-H", "-o", "health", pool)
			if err != nil {
				fail(fmt.Sprintf("pool %s health check failed: %v", pool, err))
				continue
			}
			if health != "ONLINE" {
				fail(fmt.Sprintf("pool %s health %s", pool, health))
				continue
			}
			notes = append(notes, fmt.Sprintf("pool %s ONLINE", pool))
		}
	}

	if err := run.Run("systemctl", "is-failed", "--quiet", "rtslib-fb-targetctl.service"); err == nil {
		fail("rtslib-fb-targetctl.service failed: LIO config not restored")
	}

	targets := targetCount(cfg.SaveConfig)
	if targets > 0 {
		conn, err := net.DialTimeout("tcp", cfg.Portal, 3*time.Second)
		if err != nil {
			fail(fmt.Sprintf("%d iSCSI target(s), portal %s not listening: %v", targets, cfg.Portal, err))
		} else {
			_ = conn.Close()
			notes = append(notes, fmt.Sprintf("%d iSCSI target(s), portal listening on %s", targets, portalPort(cfg.Portal)))
		}
	}

	prefix := "healthy"
	out := stdout
	errOut := error(nil)
	if unhealthy {
		prefix = "UNHEALTHY"
		out = stderr
		errOut = ErrUnhealthy
	}
	line := fmt.Sprintf("%s: %s\n", prefix, strings.Join(notes, "; "))
	if err := writeStatus(cfg.StatusFile, line); err != nil {
		return err
	}
	_, _ = fmt.Fprint(out, line)
	return errOut
}

func normalize(cfg Config) Config {
	if cfg.SaveConfig == "" {
		cfg.SaveConfig = defaultSaveConfig
	}
	if cfg.StatusFile == "" {
		cfg.StatusFile = defaultStatusFile
	}
	if cfg.Portal == "" {
		cfg.Portal = defaultPortal
	}
	return cfg
}

func targetCount(path string) int {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return 0
	}
	var saveConfig struct {
		Targets []json.RawMessage `json:"targets"`
	}
	if err := json.Unmarshal(data, &saveConfig); err != nil {
		return 0
	}
	return len(saveConfig.Targets)
}

func portalPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return port
}

func writeStatus(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
