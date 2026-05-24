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
| `../node-init/` | Go helper built into image targets for early-boot node initialization. |
| `../artifacts/` | Ignored local bootable artifact output. |
| `../Earthfile` | Earthly targets for reproducible image-build validation and artifact generation. |
| `Dockerfile` | Image-build Dockerfile that turns Ubuntu into a Kairos+k3s OCI image. |
| `cmd/image-build/` | Go CLI for target planning, OCI builds, artifact builds, and validation. |
| `../tools/cmd/k2-tools/flash.go` | CM4/ComputeBlade eMMC flasher; invoke as `k2-tools flash rpi4cb`. Replaces the legacy `scripts/flash-rpi4cb-macos.sh`. |

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
`kairosModel: generic` for QEMU-backed VMs on x86 and ARM hosts.

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

Run the CLI from source during development:

```sh
(cd kairos/image-build && go run ./cmd/image-build --help)
```

Build a standalone `k2-image-build` binary when you want a local executable:

```sh
(cd kairos/image-build && go build -o /tmp/k2-image-build ./cmd/image-build)
/tmp/k2-image-build --help
```

Build and load an OCI image:

```sh
(cd kairos/image-build && go run ./cmd/image-build build oci ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

Build the raw artifact from the locally loaded image:

```sh
(cd kairos/image-build && go run ./cmd/image-build build artifact ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

Build a generic QEMU raw artifact:

```sh
(cd kairos/image-build && go run ./cmd/image-build build artifact ubuntu-24.04-standard-amd64-qemu-k3s)
(cd kairos/image-build && go run ./cmd/image-build build artifact ubuntu-24.04-standard-arm64-qemu-k3s)
```

QEMU disk and guest-agent behavior is documented in the
[QEMU hardware overlay README](overlays/hardware/qemu/README.md).

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

Resolved target image tags include the base OS, Kairos version, architecture,
hardware profile, k3s version, and K2 image revision. For example:

```text
ghcr.io/wyvernzora/k2-kairos:ubuntu-24.04-standard-v4.1.0-arm64-rpi4cb-k3s-v1.36.0-k3s1-rev0
```

## Overlays

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

Hardware-specific behavior belongs in the hardware overlay README:

- [QEMU](overlays/hardware/qemu/README.md)
- [Raspberry Pi ComputeBlade](overlays/hardware/rpi4cb/README.md)

Kubernetes-specific image content belongs in the Kubernetes overlay README:

- [K3s](overlays/kubernetes/k3s/README.md)

## CI

`.github/workflows/kairos-image-build.yaml` validates enabled targets from
`../targets.yaml`. Pull requests run Go checks, target planning, OCI build and
inspection, and vulnerability scans, but they do not build or retain raw
bootable image artifacts.

Pushes to `main` and manual workflow dispatches additionally build raw bootable
artifacts, push OCI images to GHCR, publish bootable artifacts to S3, update a
per-target `latest/<target>/manifest.json` pointer, and prune older
`images/<target>/<git-sha>/` prefixes for the same target.

The workflow uses the pinned Earthly CI image to run the same reproducible
artifact target used locally:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=<target>
```

That Earthly target builds the target OCI image in a Linux Docker environment,
runs AuroraBoot, applies raw overlay patches, writes checksums and the artifact
manifest, and runs artifact inspection before exporting compressed artifacts to
`kairos/artifacts/`.

Only metadata is uploaded as a GitHub Actions artifact:

- `plan.json`
- `artifact-manifest.json` for non-PR artifact builds
- `publish-manifest.json` for non-PR S3 publication
- `SHA256SUMS` for non-PR artifact builds

Configure S3 publishing with repository variables:

| Variable | Required | Purpose |
| --- | --- | --- |
| `K2_KAIROS_IMAGE_AWS_REGION` | Yes | AWS region containing the artifact bucket. |
| `K2_KAIROS_IMAGE_AWS_ROLE_ARN` | Yes | IAM role assumed by GitHub Actions through OIDC. |
| `K2_KAIROS_IMAGE_BUCKET` | Yes | S3 bucket for bootable image artifacts. |
| `K2_KAIROS_IMAGE_PREFIX` | No | Optional key prefix inside the bucket. |
| `K2_KAIROS_IMAGE_KEEP` | No | Builds to retain per target. Defaults to `3`. |
| `K2_KAIROS_IMAGE_STORAGE_CLASS` | No | Storage class for image blobs. Defaults to `ONEZONE_IA`. |
