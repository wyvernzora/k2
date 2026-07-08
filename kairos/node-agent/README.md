# K2 Node Agent

`k2-node-agent` is a small Go helper baked into K2 Kairos images for node-local
runtime work. It is the K2 counterpart to `kairos-agent`: image-baked, local to
the node, and used by Kairos stages or systemd units rather than by cluster
provisioning clients.

The binary is not a cluster provisioner. Client-side provisioning is handled by
[`k2-tools`](../../tools/README.md).

## Build And Test

```sh
cd kairos/node-agent
go test ./...
go vet ./...
go run ./cmd/k2-node-agent --help
```

Build the binary directly:

```sh
(cd kairos/node-agent && go build -o /tmp/k2-node-agent ./cmd/k2-node-agent)
```

Image builds compile and install this helper into the OCI rootfs. See
[`kairos`](../README.md) for the image build flow.

## Commands

### `setup-persistence`

Prepare or verify Kairos persistent storage.

```sh
k2-node-agent setup-persistence [flags]
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
| `--verify-only` | `false` | Skip preparation and only verify `/usr/local` before writing the marker. |

Examples:

```sh
# Require a dedicated NVMe disk for persistence.
k2-node-agent setup-persistence --disk /dev/nvme0n1 --mode required

# Require any non-boot second disk for persistence.
k2-node-agent setup-persistence --disk auto --mode required

# Prefer a second disk, but keep and grow the original persistent partition if
# no suitable second disk exists.
k2-node-agent setup-persistence --disk auto --mode optional

# Verify the active mount after Kairos has mounted COS_PERSISTENT.
k2-node-agent setup-persistence --verify-only --mode required
```

`setup-persistence` uses the Kairos filesystem label contract:

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

- [QEMU overlay](../overlays/hardware/qemu/README.md)
- [Raspberry Pi ComputeBlade overlay](../overlays/hardware/rpi4cb/README.md)

### `storage-health`

Report ZFS and iSCSI storage health without taking corrective action.

```sh
k2-node-agent storage-health [flags]
```

Important flags:

| Flag | Default | Purpose |
| --- | --- | --- |
| `--saveconfig` | `/etc/rtslib-fb-target/saveconfig.json` | rtslib saveconfig path used to count configured targets. |
| `--status-file` | `/run/k2-storage-health/status` | Status file to write. |
| `--portal` | `127.0.0.1:3260` | iSCSI portal address to probe when targets exist. |

The command writes `healthy: <notes>` or `UNHEALTHY: <notes>` to the status
file and stdout or stderr. It exits non-zero only for unhealthy storage.

## Runtime Checks

After a node boots, verify persistence with:

```sh
lsblk -o NAME,SIZE,FSTYPE,LABEL,UUID,MOUNTPOINTS
findmnt /usr/local
test -f /usr/local/.state/.k2-persistent-ok
cat /run/k2-storage-health/status
```

Hardware overlays may require additional checks; keep those details in the
overlay README.
