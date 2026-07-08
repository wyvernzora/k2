# rpi4cb Hardware Overlay

Raspberry Pi CM4 ComputeBlade hardware overlay.

This overlay keeps only hardware-specific raw partition changes: enabling the
CM4 PCIe lane and shrinking the eMMC `COS_PERSISTENT` placeholder. Persistent
storage setup is role-owned by `role/k8s`.

## Files

| Path | Purpose |
| --- | --- |
| `raw/COS_GRUB/extraconfig.txt` | Enables CM4 PCIe so NVMe appears. |
| `raw/COS_OEM/01_reset.yaml.patch` | Sets the eMMC `COS_PERSISTENT` placeholder size to 500 MiB. |
| `overlay.yaml` | Declares raw artifact inspection expectations. |

Run the checks with:

```sh
./tools/k2-tools image inspect oci ubuntu-24.04-arm64-rpi4cb-k8s
./tools/k2-tools image inspect artifact ubuntu-24.04-arm64-rpi4cb-k8s
```
