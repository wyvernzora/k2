# qemu Hardware Overlay

This overlay defines the K2 hardware profile for local QEMU/KVM test VMs.

It intentionally avoids platform-specific boot or persistence customization.
The VM target should behave like a generic Kairos disk image with the shared K2
K3s invariant overlay applied, making it useful for testing the provisioning
flow without reflashing Raspberry Pi ComputeBlade hardware.

## Files

| Path | Purpose |
| --- | --- |
| `overlay.yaml` | Declares inspection expectations for the generic VM profile. |

## Inspection Expectations

`overlay.yaml` asks the image-build CLI to verify that Raspberry Pi
ComputeBlade-specific OEM and raw-patch files are not present.

Run the checks with:

```sh
(cd kairos/image-build && go run ./cmd/image-build inspect oci ubuntu-24.04-standard-amd64-qemu-k3s)
(cd kairos/image-build && go run ./cmd/image-build inspect artifact ubuntu-24.04-standard-amd64-qemu-k3s)
```
