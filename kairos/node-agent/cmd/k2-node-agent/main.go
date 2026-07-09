package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/health"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/metrics"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/storage"
)

type cli struct {
	SetupPersistence setupPersistenceCommand `cmd:"" name:"setup-persistence" help:"Prepare or verify Kairos persistent storage."`
	StorageHealth    storageHealthCommand    `cmd:"" name:"storage-health" help:"Report ZFS and iSCSI storage health."`
	Metrics          metricsCommand          `cmd:"" name:"metrics" help:"Expose K2 storage appliance Prometheus metrics."`
}

type setupPersistenceCommand struct {
	Disk        string `default:"auto" help:"Target disk path, or auto to choose a non-boot disk."`
	Mode        string `default:"optional" enum:"optional,required" help:"Whether missing second-disk provisioning is optional or required."`
	OldLabel    string `name:"old-label" default:"COS_PERSIST_OLD" help:"Label to apply to the original persistent partition."`
	Marker      string `default:".k2-persistent-ok" help:"Marker file to write under /usr/local/.state."`
	LogPrefix   string `name:"log-prefix" default:"kairos-persistent" help:"Prefix for stdout, syslog, and kernel log messages."`
	WaitSeconds int    `name:"wait-seconds" default:"30" help:"Seconds to wait for target block devices."`
	VerifyOnly  bool   `name:"verify-only" help:"Only verify /usr/local and write the marker file."`
}

type storageHealthCommand struct {
	SaveConfig string `name:"save-config" default:"/etc/rtslib-fb-target/saveconfig.json" help:"rtslib saveconfig path."`
	StatusFile string `name:"status-file" default:"/run/k2-storage-health/status" help:"Status file to write."`
	Portal     string `default:"127.0.0.1:3260" help:"iSCSI portal address to probe."`
}

type metricsCommand struct {
	TextfileDir string `default:"/var/lib/prometheus/node-exporter" env:"K2_NODE_AGENT_METRICS_TEXTFILE_DIR" help:"node_exporter textfile collector directory to write k2.prom into."`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, health.ErrUnhealthy) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "k2-node-agent: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var cli cli
	parser, err := kong.New(&cli,
		kong.Name("k2-node-agent"),
		kong.Description("K2 node agent: runtime helpers for K2 Kairos images (counterpart to kairos-agent)."),
		kong.UsageOnError(),
	)
	if err != nil {
		return err
	}
	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}
	return ctx.Run()
}

func (cmd setupPersistenceCommand) Run() error {
	mode, err := storage.ParseMode(cmd.Mode)
	if err != nil {
		return err
	}
	return storage.Run(storage.Config{
		Disk:        cmd.Disk,
		Required:    mode.Required(),
		OldLabel:    cmd.OldLabel,
		Marker:      cmd.Marker,
		LogPrefix:   cmd.LogPrefix,
		WaitSeconds: cmd.WaitSeconds,
		VerifyOnly:  cmd.VerifyOnly,
	})
}

func (cmd storageHealthCommand) Run() error {
	return health.Run(health.Config{
		SaveConfig: cmd.SaveConfig,
		StatusFile: cmd.StatusFile,
		Portal:     cmd.Portal,
	})
}

func (cmd metricsCommand) Run() error {
	return metrics.Run(metrics.Config{
		TextfileDir: cmd.TextfileDir,
		Debug:       os.Stderr,
	})
}
