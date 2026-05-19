# K2 Kairos Image Build

This directory contains the self-contained build tooling and overlay inputs for
K2 Kairos images. Cluster-agnostic target configuration still lives one level
up in `kairos/`.

## Layout

| Path | Purpose |
| --- | --- |
| `../versions.env` | Shared version pins for Kairos, kairos-init, AuroraBoot, Ubuntu, k3s, and GHCR image name. |
| `../targets.yaml` | Build matrix for enabled and planned image targets. |
| `overlays/` | Optional reviewable OCI/raw overlays plus overlay inspection metadata. |
| `../artifacts/` | Ignored local bootable artifact output. |
| `../Earthfile` | Earthly targets for reproducible image-build validation and artifact generation. |
| `Dockerfile` | Image-build Dockerfile that turns Ubuntu into a Kairos+k3s OCI image. |
| `cmd/image-build/` | Go CLI for target planning, OCI builds, artifact builds, and validation. |
| `scripts/flash-rpi4cb-macos.sh` | Host-local flashing helper for CM4 eMMC exposed through rpiboot on macOS. |

## Target Strategy

The images are cluster-light. They include Ubuntu, Kairos, k3s, hardware
defaults for the selected target, and invariant K2 K3s server config, but do
not enable the K3s service or bake cluster identity, node identity, or active
cluster-specific K3s config. Future baked content should be added by declaring
overlays in `targets.yaml`, not by making one-off Dockerfile edits.

`hardware` is the K2 hardware profile used in tags and overlay selection.
`kairosModel` is the Kairos model passed to `kairos-init`. For example,
`rpi4cb` still uses `kairosModel: rpi4` because the ComputeBlade nodes are CM4
systems using the Kairos Raspberry Pi 4 boot model. The `qemu` targets use
`kairosModel: generic` for local VM provisioning tests on x86 and ARM hosts.

Node roles are not part of the image-build contract. Bootstrap and join intent
comes from files written by the node provisioner before it activates K3s.

Never bake secrets, 1Password material, k3s tokens, private keys, cluster CA
private material, hostnames, node IPs, or VIP ownership into images.

## Local Build

Install the required tools first:

```sh
docker --version
xz --version
```

Build and load the PoC OCI image:

```sh
(cd kairos/image-build && go run ./cmd/image-build build oci ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

Build the raw artifact from the locally loaded image:

```sh
(cd kairos/image-build && go run ./cmd/image-build build artifact ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

Build a generic QEMU raw artifact for local VM provisioning tests:

```sh
(cd kairos/image-build && go run ./cmd/image-build build artifact ubuntu-24.04-standard-amd64-qemu-k3s)
(cd kairos/image-build && go run ./cmd/image-build build artifact ubuntu-24.04-standard-arm64-qemu-k3s)
```

QEMU targets generate 16 GiB disks with an 8 GiB Kairos state partition, leaving
the remainder for persistent K3s state, containerd, kubelet, and repeated
experiments.

Build the OCI image, raw artifact, raw patches, checksums, manifest, and
inspection inside Earthly's Linux/Docker environment:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-24.04-standard-arm64-rpi4cb-k3s
```

This is the preferred artifact path because all raw image mutation happens on
Linux-local storage before Earthly exports finished artifacts to
`kairos/artifacts/<target>/`. The uncompressed `.raw` file is kept only as a
build intermediate; exported artifacts use `.raw.xz` plus manifest/checksum
metadata. macOS/Docker Desktop raw-image writeback is not considered an
authoritative artifact build path. The Earthly target also exports and imports a
Docker BuildKit local cache between runs, so the expensive OCI build layers do
not have to start cold every time.

Inspect the artifact:

```sh
(cd kairos/image-build && go run ./cmd/image-build inspect artifact ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

Inspect the locally loaded OCI image against overlay/target expectations:

```sh
(cd kairos/image-build && go run ./cmd/image-build inspect oci ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

Push an OCI image after a local build succeeds:

```sh
(cd kairos/image-build && go run ./cmd/image-build build oci ubuntu-24.04-standard-arm64-rpi4cb-k3s --push)
```

Preview the resolved build plan:

```sh
(cd kairos/image-build && go run ./cmd/image-build plan ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

The PoC target produces this image tag:

```text
ghcr.io/wyvernzora/k2-kairos:ubuntu-24.04-standard-v4.1.0-arm64-rpi4cb-k3s-v1.36.0-k3s1-rev0
```

## rpi4cb Hardware Defaults

The `rpi4cb` target is for Raspberry Pi CM4 modules on ComputeBlade hardware.
It bakes in:

- `COS_GRUB/extraconfig.txt` with `[cm4] dtparam=pciex1`, applied from the
  overlay's `raw/COS_GRUB/extraconfig.txt` after AuroraBoot creates the raw
  image.
- `/system/oem/05-rpi4cb-nvme-persistent.yaml`, which runs during
  `rootfs.before` and is copied from the overlay's
  `oci/system/oem/05-rpi4cb-nvme-persistent.yaml` into the OCI rootfs.

The NVMe persistent setup is intentionally destructive for `/dev/nvme0n1` when
the disk does not already contain an NVMe-backed `COS_PERSISTENT` filesystem.
This hardware profile assumes the NVMe is dedicated to Kairos mutable state.
Any non-NVMe filesystem labeled `COS_PERSISTENT` is relabeled
`COS_PERSIST_OLD` before Kairos applies its immutable rootfs layout, avoiding
duplicate persistent labels.

With `COS_PERSISTENT` on NVMe, Kairos' default bind mounts keep high-write paths
such as `/var/lib/rancher`, `/var/lib/containerd`, `/var/lib/kubelet`,
`/var/lib/etcd`, and `/var/log` on NVMe-backed `/usr/local/.state`.

The rpi4cb raw artifact also applies
`raw/COS_OEM/01_reset.yaml.patch` to AuroraBoot's generated `/oem/01_reset.yaml`
so the first recovery boot creates only a small eMMC `COS_PERSISTENT`
placeholder. The real persistent partition is prepared on NVMe during the first
active boot, and the eMMC placeholder is relabeled to `COS_PERSIST_OLD`.

Overlay content is split by destination:

- `oci/` is copied into the OCI rootfs.
- `raw/<PARTITION_LABEL>/` is applied to generated raw image partitions.
- `overlay.yaml` declares generic inspection expectations for the overlay.

Regular raw files are copied into the matching raw image partition, while
`.yaml.patch` and `.json.patch` sidecars apply strict JSON Patch operations to
existing structured files. The artifact builder runs these patches after
AuroraBoot emits the raw image and before compression/checksum generation. The
Go artifact inspector validates the declared raw patch outcomes and raw
inspection expectations through a containerized partition inspection path. The
Go OCI inspector validates rootfs expectations by running generic Docker checks
against the resolved image tag.

## CI

`.github/workflows/kairos-image-build.yaml` builds enabled targets from
`../targets.yaml`, pushes OCI images to GHCR, and uploads bootable artifacts plus
checksums.

The workflow uses the pinned Earthly CI image to run the same reproducible
artifact target used locally:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=<target>
```

That Earthly target builds the target OCI image in a Linux Docker environment,
runs AuroraBoot, applies raw overlay patches, writes checksums and the artifact
manifest, and runs artifact inspection before exporting compressed artifacts to
`kairos/artifacts/`.
