# Migrate a Legacy Boot Layout

Use this one-shot recovery-mode procedure on older K2 nodes that still have a
boot-disk partition labelled `COS_PERSIST_OLD`. It removes that obsolete
partition and, when necessary, expands `COS_STATE` to 8192 MiB. This applies to
both QEMU and Raspberry Pi nodes.

The migration discovers partitions by filesystem label, not device name. It
also requires the expected GPT partition labels (`state` and `persistent`) and
refuses to proceed if the external `COS_PERSISTENT` filesystem shares the boot
disk. The node's persistent workload data is not modified.

## Safety Model

- Run discovery mode on the active node first. It performs no writes.
- Applying the migration is allowed only in Kairos recovery mode and only when
  `COS_STATE` is unmounted.
- A successful recovery run selects the active boot entry and reboots.
- A failed recovery run stays in recovery and preserves its log on `COS_OEM`.
- The script supports only 512-byte logical sectors and ext2/3/4 `COS_STATE`
  filesystems. Any other layout requires a separate procedure.

Before the first Raspberry Pi apply, prove the full recovery workflow on a
QEMU node. Legacy Pi recovery images may not have remote operator SSH, so have
UART access available before selecting recovery on a Pi.

## Install the Temporary Bundle

Copy the script and yip cloud config to the node's writable OEM partition:

```sh
scp kairos/docs/legacy-boot-layout-migration.sh \
  kairos/docs/legacy-boot-layout-migration.yaml \
  kairos@<node>:/tmp/

ssh kairos@<node> 'sudo install -m 0755 \
  /tmp/legacy-boot-layout-migration.sh \
  /oem/k2-legacy-boot-layout.sh && sudo install -m 0644 \
  /tmp/legacy-boot-layout-migration.yaml \
  /oem/97-k2-legacy-boot-layout.yaml'
```

## Active-Boot Discovery Dry Run

Run the script directly while the node remains active:

```sh
ssh kairos@<node> \
  'sudo /oem/k2-legacy-boot-layout.sh --mode dry-run'
```

Confirm that:

- `COS_STATE` and `COS_PERSIST_OLD` are on the same boot disk;
- protected `COS_PERSISTENT` has a different parent disk;
- the planned state target is 8192 MiB;
- QEMU nodes with a 4 GiB state plan both removal and growth;
- Pi nodes already at 8 GiB state plan removal only.

Stop if discovery reports an error or differs from the expected layout.

## Recovery Dry Run

First request a no-write recovery boot:

```sh
ssh kairos@<node> 'printf "dry-run\n" | sudo tee \
  /oem/k2-legacy-boot-layout.request >/dev/null && \
  sudo kairos-agent bootentry --select recovery && sudo reboot'
```

The yip stage runs in recovery, verifies that `COS_STATE` is unmounted, writes
`/oem/k2-legacy-boot-layout.log`, then returns to active boot. After the node
returns, inspect the log and confirm the partition table is unchanged:

```sh
ssh kairos@<node> 'sudo cat /oem/k2-legacy-boot-layout.log; \
  lsblk -o NAME,PATH,SIZE,TYPE,FSTYPE,LABEL,PARTLABEL,MOUNTPOINTS'
```

## Apply

After the recovery dry run succeeds, request the actual migration:

```sh
ssh kairos@<node> 'printf "apply\n" | sudo tee \
  /oem/k2-legacy-boot-layout.request >/dev/null && \
  sudo kairos-agent bootentry --select recovery && sudo reboot'
```

The node should automatically return to active boot. Verify the result:

```sh
ssh kairos@<node> 'sudo cat /oem/k2-legacy-boot-layout.log; \
  lsblk -o NAME,PATH,SIZE,TYPE,FSTYPE,LABEL,PARTLABEL,MOUNTPOINTS; \
  findmnt /run/initramfs/cos-state; findmnt /usr/local'
```

Expected results:

- `COS_PERSIST_OLD` is absent;
- `COS_STATE` is at least 8 GiB and mounted normally;
- `COS_PERSISTENT` is unchanged and still backs `/usr/local`.

Remove the temporary bundle after verification:

```sh
ssh kairos@<node> 'sudo rm -f \
  /oem/97-k2-legacy-boot-layout.yaml \
  /oem/k2-legacy-boot-layout.sh \
  /oem/k2-legacy-boot-layout.request \
  /oem/k2-legacy-boot-layout.success'
```

Keep the log until the node upgrade is complete.
