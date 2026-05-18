# K2 Kairos Images

This directory defines the Kairos images used to bootstrap K2 nodes.
It is the image configuration layer: targets, version pins, hardware overlays,
and Earthly targets live here. The build-system implementation lives separately
under `image-build/` so it can be moved into its own repository later if that
becomes useful.

## Layout

| Path | Purpose |
| --- | --- |
| `targets.yaml` | Enabled and planned image targets. Each target selects hardware, Kairos model, artifact types, overlays, and raw artifact sizing. |
| `versions.env` | Pinned Kairos, kairos-init, AuroraBoot, Ubuntu, k3s, image revision, and registry values. |
| `overlays/` | Reviewable target content. `oci/` content is baked into OCI images, `raw/` content patches generated disk partitions, and `overlay.yaml` declares inspection expectations. |
| `image-build/` | Self-contained Go CLI and Dockerfile for planning, building, patching, inspecting, and flashing images. |
| `Earthfile` | Reproducible Earthly targets for Go validation, OCI builds, raw artifact generation, patching, and inspection. |

## Current Target

The active target is:

```text
ubuntu-24.04-standard-arm64-rpi4cb-k3s
```

It builds an Ubuntu 24.04, Kairos v4.1.0, arm64 Raspberry Pi 4 model image with
k3s v1.36.0+k3s1. The hardware profile is `rpi4cb`, meaning Raspberry Pi CM4 on
ComputeBlade. The target is intentionally cluster-light: it includes the OS,
k3s, hardware defaults, and the invariant K2 K3s provisioning contract, but not
node identity, cluster tokens, VIP ownership, hostnames, private keys, active
K3s config, or other secrets.

## Build Flow

The preferred local artifact path is Earthly:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-24.04-standard-arm64-rpi4cb-k3s
```

That target runs the Go tests and vet checks, builds the OCI image, validates
OCI expectations, runs AuroraBoot, applies raw partition patches, writes
checksums and a manifest, validates the raw artifact, then exports the
compressed image and metadata under `kairos/artifacts/<target>/`.

Useful direct CLI commands:

```sh
(cd kairos/image-build && go run ./cmd/image-build plan ubuntu-24.04-standard-arm64-rpi4cb-k3s)
(cd kairos/image-build && go run ./cmd/image-build inspect oci ubuntu-24.04-standard-arm64-rpi4cb-k3s)
(cd kairos/image-build && go run ./cmd/image-build inspect artifact ubuntu-24.04-standard-arm64-rpi4cb-k3s)
```

## Overlay Contract

Selected overlays are applied in target order:

- `oci/` is copied into the OCI root filesystem.
- `raw/<PARTITION_LABEL>/` is applied to generated raw image partitions after
  AuroraBoot creates the disk image.
- `.yaml.patch` and `.json.patch` files under `raw/` are strict JSON Patch
  operations against existing structured files.
- `overlay.yaml` declares inspection expectations consumed by the image-build
  CLI to verify generated images and artifacts.

## Safety Notes

Do not bake secrets, cluster credentials, node identity, hostnames, private IP
assignments, or private key material into these images.
