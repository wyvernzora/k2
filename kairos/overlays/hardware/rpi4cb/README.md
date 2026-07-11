# rpi4cb Hardware Overlay

Raspberry Pi CM4 ComputeBlade hardware overlay.

This overlay keeps only the hardware-specific raw partition change that enables
the CM4 PCIe lane. Persistent storage and the boot-disk layout are role-owned by
`role/k8s`.

## Files

| Path | Purpose |
| --- | --- |
| `raw/COS_GRUB/extraconfig.txt` | Enables CM4 PCIe so NVMe appears. |
| `overlay.yaml` | Declares raw artifact inspection expectations. |

Run the checks with:

```sh
./tools/k2-tools image inspect oci ubuntu-26.04-arm64-rpi4cb-k8s
./tools/k2-tools image inspect artifact ubuntu-26.04-arm64-rpi4cb-k8s
```
