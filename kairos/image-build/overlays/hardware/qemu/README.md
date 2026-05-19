# qemu Hardware Overlay

This overlay defines the K2 hardware profile for local QEMU/KVM test VMs.

It keeps the VM target close to a generic Kairos disk image, but adds one
test-friendly persistence behavior: if a second disk is present during active
boot, that disk becomes `COS_PERSISTENT`; if no second disk is present, the
original persistent partition is kept and grown to the end of its disk.

## Files

| Path | Purpose |
| --- | --- |
| `oci/system/oem/05-qemu-persistent.yaml` | Baked into the OCI rootfs as `/system/oem/05-qemu-persistent.yaml`. On active boot, it invokes `k2-node-init storage` to move `COS_PERSISTENT` to a second disk when available, otherwise grow the existing persistent partition. |
| `overlay.yaml` | Declares inspection expectations for the QEMU profile. |

## Inspection Expectations

`overlay.yaml` asks the image-build CLI to verify that:

- the QEMU persistent OEM file is present;
- Raspberry Pi ComputeBlade-specific OEM files are not present;
- `k2-node-init` and required disk utilities are available in the image.

Run the checks with:

```sh
(cd kairos/image-build && go run ./cmd/image-build inspect oci ubuntu-24.04-standard-amd64-qemu-k3s)
(cd kairos/image-build && go run ./cmd/image-build inspect artifact ubuntu-24.04-standard-amd64-qemu-k3s)
```
