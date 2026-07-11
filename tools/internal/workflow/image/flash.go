package image

import (
	"context"
	"os"
	"time"

	"github.com/wyvernzora/k2/tools/internal/step/flash"
)

func flashRegistration() Registration {
	return Registration{Name: "flash", Help: "Flash Kairos images to hardware.", Order: 40, Command: &flashCmd{}}
}

type flashCmd struct {
	Rpi4cb flashRpi4cbCmd `cmd:"" name:"rpi4cb" help:"Flash a Kairos rpi4cb image onto a ComputeBlade / CM4."`
}

type flashRpi4cbCmd struct {
	EMMC           string        `name:"emmc" env:"K2_FLASH_EMMC" help:"Disk identifier of the eMMC target (e.g. disk4). Skips rpiboot auto-detect when set."`
	NVMe           string        `name:"nvme" env:"K2_FLASH_NVMe" help:"Disk identifier of the NVMe target (e.g. disk5). Only consulted with --emmc + --zero-nvme."`
	ZeroNVMe       bool          `name:"zero-nvme" env:"K2_FLASH_ZERO_NVME" help:"Zero the first 64 MiB of the NVMe so its boot metadata can't shadow the freshly-flashed eMMC."`
	SkipVerify     bool          `name:"skip-verify" env:"K2_FLASH_SKIP_VERIFY" help:"Skip the post-write SHA256 readback."`
	SkipRpiboot    bool          `name:"skip-rpiboot" env:"K2_FLASH_SKIP_RPIBOOT" help:"Do not invoke rpiboot; auto-detect among currently-attached external disks."`
	Yes            bool          `name:"yes" short:"y" env:"K2_FLASH_YES" help:"Bypass the FLASH confirmation prompt. Required for non-TTY invocation."`
	RpibootTimeout time.Duration `name:"rpiboot-timeout" env:"K2_FLASH_RPIBOOT_TIMEOUT" default:"60s" help:"How long to wait for new disks to appear after rpiboot exits."`
}

func (c *flashRpi4cbCmd) Run(ctx *Runtime) error {
	runner := flash.Runner{
		RepoRoot: ctx.RepoRoot,
		Reporter: currentReporter(),
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
	return runner.Rpi4cb(context.Background(), flash.Rpi4cbOptions{
		EMMCDiskID:     c.EMMC,
		NVMeDiskID:     c.NVMe,
		ZeroNVMe:       c.ZeroNVMe,
		SkipVerify:     c.SkipVerify,
		SkipRpiboot:    c.SkipRpiboot,
		Yes:            c.Yes,
		RpibootTimeout: c.RpibootTimeout,
	})
}
