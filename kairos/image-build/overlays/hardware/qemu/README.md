# qemu Hardware Overlay

This overlay defines the K2 hardware profile for local QEMU/KVM test VMs.

It keeps the VM target close to a generic Kairos disk image, but requires one
test-friendly persistence behavior: a second disk must be present during active
boot, and that disk becomes `COS_PERSISTENT`. The boot disk keeps only a small
placeholder persistent partition so QEMU artifacts stay compact.

## Files

| Path | Purpose |
| --- | --- |
| `oci/system/oem/05-qemu-persistent.yaml` | Baked into the OCI rootfs as `/system/oem/05-qemu-persistent.yaml`. On active boot, it invokes `k2-node-init storage` to move `COS_PERSISTENT` to a required second disk. |
| `oci/system/oem/07-qemu-guest-agent.yaml` | Baked into the OCI rootfs as `/system/oem/07-qemu-guest-agent.yaml`. On active boot, it enables `qemu-guest-agent.service` when `qemu-ga` is present. |
| `overlay.yaml` | Declares inspection expectations for the QEMU profile. |

Serial login is intentionally not baked into the QEMU image. Production worker
nodes should not expose a persistent console listener by default; local test
harnesses can attach a hypervisor console when needed.

## Inspection Expectations

`overlay.yaml` asks the image-build CLI to verify that:

- the QEMU persistent OEM file is present;
- the QEMU guest agent OEM file is present;
- the QEMU serial console OEM file is not present;
- Raspberry Pi ComputeBlade-specific OEM files are not present;
- `k2-node-init`, `qemu-ga`, and required disk utilities are available in the image.

Run the checks with:

```sh
(cd kairos/image-build && go run ./cmd/image-build inspect oci ubuntu-24.04-standard-amd64-qemu-k3s)
(cd kairos/image-build && go run ./cmd/image-build inspect artifact ubuntu-24.04-standard-amd64-qemu-k3s)
```
