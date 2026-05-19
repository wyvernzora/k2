package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/wyvernzora/k2/kairos/node-init/internal/storage"
)

type cli struct {
	Storage storageCommand `cmd:"" help:"Prepare or verify Kairos persistent storage."`
}

type storageCommand struct {
	Disk         string `default:"auto" help:"Target disk path, or auto to choose a non-boot disk."`
	Mode         string `default:"optional" enum:"optional,required" help:"Whether missing second-disk provisioning is optional or required."`
	OldLabel     string `name:"old-label" default:"COS_PERSIST_OLD" help:"Label to apply to the original persistent partition."`
	Marker       string `default:".k2-persistent-ok" help:"Marker file to write under /usr/local/.state."`
	LogPrefix    string `name:"log-prefix" default:"kairos-persistent" help:"Prefix for stdout, syslog, and kernel log messages."`
	WaitSeconds  int    `name:"wait-seconds" default:"30" help:"Seconds to wait for target block devices."`
	VerifyPrefix string `name:"verify-prefix" help:"Require /usr/local source to resolve with this prefix during verify."`
	VerifyOnly   bool   `name:"verify-only" help:"Only verify /usr/local and write the marker file."`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "k2-node-init: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var cli cli
	parser, err := kong.New(&cli,
		kong.Name("k2-node-init"),
		kong.Description("Kairos node initialization helpers for K2 images."),
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

func (cmd storageCommand) Run() error {
	mode, err := storage.ParseMode(cmd.Mode)
	if err != nil {
		return err
	}
	return storage.Run(storage.Config{
		Disk:         cmd.Disk,
		Required:     mode.Required(),
		OldLabel:     cmd.OldLabel,
		Marker:       cmd.Marker,
		LogPrefix:    cmd.LogPrefix,
		WaitSeconds:  cmd.WaitSeconds,
		VerifyPrefix: cmd.VerifyPrefix,
		VerifyOnly:   cmd.VerifyOnly,
	})
}
