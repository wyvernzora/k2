# K2 Kairos Images

This directory defines the Kairos images used to bootstrap K2 nodes.
It is the image configuration layer: targets, version pins, and Earthly targets
live here. Image-build implementation details, including hardware and
Kubernetes overlays, live under `image-build/`.

## Layout

| Path | Purpose |
| --- | --- |
| `targets.yaml` | Enabled and planned image targets. Each target selects hardware, Kairos model, artifact types, overlays, and raw artifact sizing. |
| `versions.env` | Pinned Kairos, kairos-init, AuroraBoot, Ubuntu, k3s, image revision, and registry values. |
| `image-build/` | Self-contained Go CLI, Dockerfile, and overlays for planning, building, patching, inspecting, and flashing images. |
| `node-init/` | Go helper baked into images for early-boot node initialization, currently persistent storage preparation. |
| `tools/` | Client-side Go CLI for provisioning clean Kairos nodes and managing local QEMU VMs. |
| `Earthfile` | Reproducible Earthly targets for Go validation, OCI builds, raw artifact generation, patching, and inspection. |

## Current Targets

The active targets in `targets.yaml` are:

```text
ubuntu-24.04-standard-amd64-qemu-k3s
ubuntu-24.04-standard-arm64-qemu-k3s
ubuntu-24.04-standard-arm64-rpi4cb-k3s
```

All active targets build Ubuntu 24.04, Kairos v4.1.0 images with k3s
v1.36.0+k3s1.

- `qemu` targets are generic Kairos VM images. Use `amd64` on x86 hosts and
  `arm64` on Apple Silicon or other ARM hosts. The same target family is used
  for local VM testing and for QEMU-backed cluster nodes in any provisioned
  role.
- `rpi4cb` is an arm64 Raspberry Pi 4 model image for Raspberry Pi CM4 modules
  on ComputeBlade.

Hardware-specific behavior lives in the hardware overlay docs:

- [QEMU overlay](image-build/overlays/hardware/qemu/README.md)
- [Raspberry Pi ComputeBlade overlay](image-build/overlays/hardware/rpi4cb/README.md)

The targets are intentionally cluster-light: they include the OS, k3s, hardware
defaults, and invariant K2 K3s server config, but do not enable the K3s service
or bake node identity, cluster tokens, VIP ownership, hostnames, private keys,
active cluster-specific K3s config, or other secrets.

## Build Flow

The preferred local artifact path is Earthly:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-24.04-standard-arm64-rpi4cb-k3s
```

For QEMU-backed VMs, build the QEMU target instead:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-24.04-standard-amd64-qemu-k3s
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-24.04-standard-arm64-qemu-k3s
```

`k2-tools vm create` prefers those local artifacts when present. If there is no
local `kairos/artifacts/<target>/*.raw.xz`, it reads the public S3
`latest/<target>/manifest.json`, downloads the matching raw artifact, verifies
its SHA256, and caches it under `.testvm/cache/artifacts/` for later VMs.

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

The Kairos Go tools are documented in their module READMEs:

- [k2-image-build](image-build/README.md) plans, builds, patches, and inspects images.
- [k2-node-init](node-init/README.md) runs inside images for early-boot node initialization.
- [k2-tools](tools/README.md) provisions clean nodes and manages local QEMU VMs.

## Provisioning

After a raw-image node boots into the installed Kairos system, use the
provisioner to write role-specific config and activate k3s:

```sh
(cd kairos/tools && go run ./cmd/k2-tools provision bootstrap --cluster-target v3 --cluster-name v3-test --host 127.0.0.1 --ssh-port 2222 --node-name v3-test-01 --operator-key-file ~/.ssh/id_ed25519.pub --extra-manifests '/path/to/bootstrap/*.yaml')
```

Use `provision render bootstrap` first when you want to inspect the bundle
locally. For local QEMU VM testing, use `k2-tools vm create --start` and
`k2-tools provision ... --test-vm <id>`.

## Image-Build Overlay Contract

Selected overlays from `image-build/overlays/` are applied in target order:

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
