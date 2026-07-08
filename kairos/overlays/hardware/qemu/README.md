# qemu Hardware Overlay

QEMU/KVM hardware overlay for K2 Kairos images.

This overlay carries QEMU guest-agent activation and declares the
`qemu-guest-agent` package through `overlay.yaml`. It does not own persistent
storage behavior; k8s images get that from `role/k8s`, while storage images
must not auto-claim non-boot disks.

## Files

| Path | Purpose |
| --- | --- |
| `oci/system/oem/07-qemu-guest-agent.yaml` | Enables `qemu-guest-agent.service` on active boot. |
| `overlay.yaml` | Declares build packages and inspection expectations. |

Run the checks with:

```sh
./tools/k2-tools image inspect oci ubuntu-24.04-amd64-qemu-k8s
./tools/k2-tools image inspect artifact ubuntu-24.04-amd64-qemu-k8s
```
