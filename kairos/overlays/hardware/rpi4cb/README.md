# rpi4cb Hardware Overlay

This overlay defines the K2 hardware profile for Raspberry Pi CM4 modules on
ComputeBlade. It assumes the node boots Kairos from eMMC and uses the on-board
NVMe drive as the Kairos `COS_PERSISTENT` filesystem for mutable state.

## Why This Exists

ComputeBlade exposes NVMe over the CM4 PCIe lane, but the shipped Kairos
Raspberry Pi boot path relies on U-Boot and did not provide a usable NVMe boot
target in our testing. The practical design is therefore:

- boot firmware, recovery, OEM config, and state image management on eMMC;
- mutable runtime persistence on NVMe;
- no k3s or container storage written to eMMC during normal operation.

Kairos already bind-mounts write-heavy paths such as `/var/lib/rancher`,
`/var/lib/containerd`, `/var/lib/kubelet`, `/var/lib/etcd`, and `/var/log`
through `COS_PERSISTENT`. Making NVMe the only filesystem labeled
`COS_PERSISTENT` moves those writes off the eMMC without hand-maintaining a
separate `/var` layout.

## Files

| Path | Purpose |
| --- | --- |
| `oci/system/oem/05-rpi4cb-nvme-persistent.yaml` | Baked into the OCI rootfs as `/system/oem/05-rpi4cb-nvme-persistent.yaml`. On active boot, it prepares `/dev/nvme0n1p1` as `COS_PERSISTENT`, relabels any eMMC `COS_PERSISTENT` to `COS_PERSIST_OLD`, and verifies `/usr/local` is NVMe-backed. |
| `raw/COS_GRUB/extraconfig.txt` | Copied into the generated `COS_GRUB` partition. It enables the CM4 PCIe lane with `[cm4] dtparam=pciex1`, which is required for the NVMe device to appear. |
| `raw/COS_OEM/01_reset.yaml.patch` | Applies a JSON Patch to AuroraBoot's generated `COS_OEM/01_reset.yaml`, adding `size: 500` to the eMMC `COS_PERSISTENT` placeholder partition. The real persistent filesystem is moved to NVMe on first active boot. |
| `overlay.yaml` | Declares OCI and raw artifact inspection expectations for this overlay. These checks are consumed by the image-build CLI to verify generated images and artifacts. |

## Boot Sequence

On first recovery boot, AuroraBoot's reset layout creates eMMC `COS_STATE` and a
small 500 MiB eMMC `COS_PERSISTENT` placeholder. On the first active boot, the
cloud-config in `05-rpi4cb-nvme-persistent.yaml` runs during `rootfs.before`,
before Kairos mounts `LABEL=COS_PERSISTENT` at `/usr/local`.

The script waits for `/dev/nvme0n1`, relabels any non-NVMe `COS_PERSISTENT` to
`COS_PERSIST_OLD`, creates or reuses `/dev/nvme0n1p1`, labels it
`COS_PERSISTENT`, and lets Kairos mount NVMe-backed persistence for the normal
state bind mounts.

This setup is intentionally destructive for `/dev/nvme0n1` when no NVMe-backed
`COS_PERSISTENT` filesystem exists. The ComputeBlade NVMe is treated as a
dedicated Kairos state disk.

## Inspection Expectations

`overlay.yaml` asks the image-build CLI to verify:

- OCI image contains `/system/oem/05-rpi4cb-nvme-persistent.yaml`;
- transitional `/system/oem/20-rpi4cb-nvme-data.yaml` is absent;
- rootfs commands used by the cloud-config are present;
- raw `COS_GRUB/extraconfig.txt` contains `dtparam=pciex1`;
- raw `COS_OEM/01_reset.yaml` contains the `COS_PERSISTENT` entry with
  `size: 500`.

Run the checks with:

```sh
(cd kairos/image-build && go run ./cmd/image-build inspect oci ubuntu-24.04-standard-arm64-rpi4cb-k3s)
(cd kairos/image-build && go run ./cmd/image-build inspect artifact ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

## Post-Boot Checks

After flashing and booting a node, the important runtime checks are:

```sh
lsblk -o NAME,SIZE,FSTYPE,LABEL,UUID,MOUNTPOINTS
findmnt /usr/local
findmnt /var/lib/rancher
findmnt /var/lib/containerd
findmnt /var/lib/kubelet
findmnt /var/log
```

`/usr/local` and the listed write-heavy paths should resolve to the NVMe-backed
filesystem labeled `COS_PERSISTENT`. The old eMMC placeholder should be labeled
`COS_PERSIST_OLD`.
