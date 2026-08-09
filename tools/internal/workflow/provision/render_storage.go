package provision

import (
	"fmt"

	"github.com/wyvernzora/k2/tools/internal/nodeconfig"
)

func (c *renderStorageCmd) Run(ctx *Runtime) error {
	if c.ClusterName == "" {
		c.ClusterName = c.ClusterTarget
	}
	node, _, err := nodeconfig.Load(ctx.RepoRoot, c.ClusterTarget, c.NodeName)
	if err != nil {
		return err
	}
	vdevs, err := parseStorageVDevs(c.PoolVDev, false)
	if err != nil {
		return err
	}
	if len(vdevs) == 0 {
		return fmt.Errorf("render storage requires --pool-vdev for the pool create path")
	}
	csiPublicKey, _, _, err := resolveCSIKey(c.CSIPublicKey)
	if err != nil {
		return err
	}
	poolKey, err := generatePoolKey()
	if err != nil {
		return err
	}
	bundle, err := buildStorageBundle(c.commonStorageFlags, node, false, vdevs, csiPublicKey, poolKey)
	if err != nil {
		return err
	}
	if err := writeStorageBundle(c.OutputDir, bundle); err != nil {
		return err
	}
	successf("wrote storage bundle to %s", c.OutputDir)
	return nil
}
