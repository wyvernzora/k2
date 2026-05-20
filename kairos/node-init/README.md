# K2 Node Init

`k2-node-init` is a small Go helper baked into K2 Kairos images for early-boot
node initialization. It currently owns persistent storage preparation for
hardware profiles that need Kairos `COS_PERSISTENT` moved or grown before the
normal active boot finishes.

The binary is not a cluster provisioner. It runs on the node from Kairos
cloud-config stages baked into hardware overlays. Client-side provisioning is
handled by [`k2-tools`](../tools/README.md).

## Build And Test

```sh
cd kairos/node-init
go test ./...
go vet ./...
go run ./cmd/k2-node-init --help
```

Build the binary directly:

```sh
(cd kairos/node-init && go build -o /tmp/k2-node-init ./cmd/k2-node-init)
```

Image builds compile and install this helper into the OCI rootfs. See
[`kairos/image-build`](../image-build/README.md) for the image build flow.

## Commands

### `storage`

Prepare or verify Kairos persistent storage.

```sh
k2-node-init storage [flags]
```

Important flags:

| Flag | Default | Purpose |
| --- | --- | --- |
| `--disk` | `auto` | Target disk path, or `auto` to choose a non-boot disk. |
| `--mode` | `optional` | `required` fails when no suitable target disk exists; `optional` falls back to the existing persistent partition. |
| `--old-label` | `COS_PERSIST_OLD` | Label applied to an old persistent partition when persistence moves elsewhere. |
| `--marker` | `.k2-persistent-ok` | Marker file written below `/usr/local/.state` after successful verification. |
| `--log-prefix` | `kairos-persistent` | Prefix for stdout, syslog, and kernel log messages. |
| `--wait-seconds` | `30` | Seconds to wait for expected block devices. |
| `--verify-prefix` | empty | Require `/usr/local` to resolve to a source path with this prefix during verification. |
| `--verify-only` | `false` | Skip preparation and only verify `/usr/local` before writing the marker. |

Examples:

```sh
# Require a dedicated NVMe disk for persistence.
k2-node-init storage --disk /dev/nvme0n1 --mode required --verify-prefix /dev/nvme

# Require any non-boot second disk for persistence.
k2-node-init storage --disk auto --mode required

# Prefer a second disk, but keep and grow the original persistent partition if
# no suitable second disk exists.
k2-node-init storage --disk auto --mode optional

# Verify the active mount after Kairos has mounted COS_PERSISTENT.
k2-node-init storage --verify-only --verify-prefix /dev/nvme
```

## Storage Behavior

`storage` uses the Kairos filesystem label contract:

- active persistent filesystem label: `COS_PERSISTENT`
- temporary new filesystem label during migration: `COS_PERSIST_NEW`
- old persistent filesystem label after migration: `COS_PERSIST_OLD`

When preparing a target disk, the helper:

1. Finds the boot disk and avoids selecting it for `auto`.
2. Selects an existing or empty non-boot target disk.
3. Creates a single ext4 partition when the target disk needs initialization.
4. Copies existing persistent data when an original `COS_PERSISTENT` exists.
5. Relabels old persistent filesystems to `COS_PERSIST_OLD`.
6. Labels the target partition `COS_PERSISTENT`.
7. Grows the filesystem to fill the partition.

When no suitable target disk exists and `--mode optional` is used, the helper
keeps the original persistent partition and grows it in place.

Hardware overlays decide whether this behavior is required or optional:

- [QEMU overlay](../image-build/overlays/hardware/qemu/README.md)
- [Raspberry Pi ComputeBlade overlay](../image-build/overlays/hardware/rpi4cb/README.md)

## Runtime Checks

After a node boots, verify persistence with:

```sh
lsblk -o NAME,SIZE,FSTYPE,LABEL,UUID,MOUNTPOINTS
findmnt /usr/local
test -f /usr/local/.state/.k2-persistent-ok
```

Hardware overlays may require additional checks; keep those details in the
overlay README.
