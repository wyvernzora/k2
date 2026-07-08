# K2 Kairos Images

This directory defines the Kairos images used to bootstrap K2 nodes.
It is the image configuration layer: targets, version pins, and Earthly targets
live here alongside the Dockerfile, image overlays, and CI helper scripts.

## Layout

| Path | Purpose |
| --- | --- |
| `targets.yaml` | Enabled and planned image targets. Each target selects hardware, Kairos model, artifact types, overlays, and raw artifact sizing. |
| `versions.env` | Pinned Kairos, kairos-init, AuroraBoot, Ubuntu, k3s, image revision, and registry values. |
| `Dockerfile` | Image-build Dockerfile that turns Ubuntu into Kairos role images. |
| `overlays/` | Optional reviewable OCI/raw overlays plus overlay inspection metadata. |
| `scripts/` | CI helpers for target selection and artifact publication. |
| `node-agent/` | Go helper baked into images as `/usr/sbin/k2-node-agent` for early-boot setup and health checks. |
| `tools/` | Presets and docs for provisioning clean Kairos nodes and managing local QEMU VMs through root `k2-tools`. |
| `Earthfile` | Compatibility wrappers around root Earthly targets for image validation and artifact generation. |

## Current Targets

The active targets in `targets.yaml` are:

```text
ubuntu-26.04-amd64-qemu-k8s
ubuntu-26.04-arm64-qemu-k8s
ubuntu-26.04-arm64-rpi4cb-k8s
ubuntu-26.04-amd64-qemu-storage
ubuntu-26.04-arm64-qemu-storage
```

All active targets build Ubuntu 24.04 Kairos images pinned by
`versions.env`; `k8s` targets include k3s, and `storage` targets are
Kubernetes-free.

- `qemu` `k8s` targets are generic Kairos VM images. Use `amd64` on x86
  hosts and `arm64` on Apple Silicon or other ARM hosts. The same target
  family is used for local VM testing and for QEMU-backed cluster nodes in
  any provisioned role.
- `rpi4cb` is an arm64 Raspberry Pi 4 model image for Raspberry Pi CM4 modules
  on ComputeBlade.
- `qemu-storage` targets are Kubernetes-free images for the ZFS + iSCSI storage
  appliance VM. They combine the shared `base`, `hardware/qemu`, and
  `role/storage` overlays.

Hardware-specific behavior lives in the hardware overlay docs:

- [QEMU overlay](overlays/hardware/qemu/README.md)
- [Raspberry Pi ComputeBlade overlay](overlays/hardware/rpi4cb/README.md)

Role-specific behavior lives in the role overlay docs:

- [k8s role overlay](overlays/role/k8s/README.md)
- [storage role overlay](overlays/role/storage/README.md)

The targets are intentionally cluster-light: they include the OS, k3s, hardware
defaults, and invariant K2 K3s server config, but do not enable the K3s service
or bake node identity, cluster tokens, VIP ownership, hostnames, private keys,
active cluster-specific K3s config, or other secrets.

Future baked content should be added by declaring overlays in `targets.yaml`,
not by making one-off Dockerfile edits.

## Build Flow

The preferred local artifact path is Earthly:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-26.04-arm64-rpi4cb-k8s
```

For QEMU-backed VMs, build the QEMU target instead:

```sh
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-26.04-amd64-qemu-k8s
earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=ubuntu-26.04-arm64-qemu-k8s
```

`./tools/k2-tools vm create` prefers those local artifacts when present. If there is no
local `kairos/artifacts/<target>/*.raw.xz`, it reads the public S3
`latest/<target>/manifest.json`, downloads the matching raw artifact, verifies
its SHA256, and caches it under `.testvm/cache/artifacts/` for later VMs.

That target runs the Go tests and vet checks, builds the OCI image, validates
OCI expectations, runs AuroraBoot, applies raw partition patches, writes
checksums and a manifest, validates the raw artifact, then exports the
compressed image and metadata under `kairos/artifacts/<target>/`.

Useful direct CLI commands:

```sh
cd tools
go build -o k2-tools ./cmd/k2-tools
cd ..
./tools/k2-tools image --help
./tools/k2-tools image plan ubuntu-26.04-arm64-rpi4cb-k8s
./tools/k2-tools image build oci ubuntu-26.04-arm64-rpi4cb-k8s
./tools/k2-tools image build artifact ubuntu-26.04-arm64-rpi4cb-k8s
./tools/k2-tools image inspect oci ubuntu-26.04-arm64-rpi4cb-k8s
./tools/k2-tools image inspect artifact ubuntu-26.04-arm64-rpi4cb-k8s
```

The Kairos Go tools are documented in their module READMEs:

- `k2-node-agent` runs inside images for early-boot setup and storage health checks.
- [k2-tools](tools/README.md) provisions clean nodes and manages local QEMU VMs.

## Provisioning

After a raw-image node boots into the installed Kairos system, use the
provisioner to write role-specific config and activate k3s:

```sh
./tools/k2-tools provision bootstrap --cluster-target v3 --cluster-name v3-test --host 127.0.0.1 --ssh-port 2222 --node-name v3-test-01 --operator-key-file ~/.ssh/id_ed25519.pub --extra-manifests '/path/to/bootstrap/*.yaml'
```

Use `provision render bootstrap` first when you want to inspect the bundle
locally. For local QEMU VM testing, use `./tools/k2-tools vm create --start` and
`./tools/k2-tools provision ... --test-vm <id>`.

## Image-Build Overlay Contract

Selected overlays from `overlays/` are applied in target order:

- `oci/` is copied into the OCI root filesystem.
- `raw/<PARTITION_LABEL>/` is applied to generated raw image partitions after
  AuroraBoot creates the disk image.
- `.yaml.patch` and `.json.patch` files under `raw/` are strict JSON Patch
  operations against existing structured files.
- `overlay.yaml` declares inspection expectations consumed by `k2-tools image`
  to verify generated images and artifacts.

Common target axes are `flavor`, `role`, `arch`, and `hardware`; `platform` and
`kubernetesDistro` are derived by the planner.

Hardware-specific behavior belongs in the hardware overlay README:

- [QEMU](overlays/hardware/qemu/README.md)
- [Raspberry Pi ComputeBlade](overlays/hardware/rpi4cb/README.md)

Role-specific image content belongs in the role overlay READMEs:

- [k8s](overlays/role/k8s/README.md)
- [storage](overlays/role/storage/README.md)

## Safety Notes

Do not bake secrets, cluster credentials, node identity, hostnames, private IP
assignments, or private key material into these images.
