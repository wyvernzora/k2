# Expand a Dedicated Persistent Disk

Use this runbook to grow the external ext4 disk labelled `COS_PERSISTENT` after
increasing the size of its backing virtual disk. K2 intentionally does not do
this automatically during boot.

This procedure does not resize the boot disk's `COS_STATE` partition. A
physical disk cannot be enlarged in place; migrate its data to a larger disk
instead.

## Preconditions

- The node uses the current K8s layout: a dedicated, non-boot persistent disk
  containing one ext4 partition labelled `COS_PERSISTENT`.
- The backing virtual disk has already been enlarged in the hypervisor.
- Important data is backed up or otherwise replaceable.
- The node has been cordoned and drained from an operator machine.

For example:

```sh
kubectl --kubeconfig="$HOME/.kube/k2/k2/kubeconfig" \
  drain <node-name> \
  --ignore-daemonsets \
  --delete-emptydir-data \
  --timeout=15m
```

Do not bypass a blocking PodDisruptionBudget merely to complete this
maintenance.

## Identify and Verify the Devices

Run these commands on the node:

```sh
set -eu

persist_part=$(readlink -f /dev/disk/by-label/COS_PERSISTENT)
persist_parent=$(lsblk -ndo PKNAME "$persist_part")
test -n "$persist_parent" || {
  echo "Cannot determine the COS_PERSISTENT parent disk" >&2
  exit 1
}
persist_disk="/dev/$persist_parent"

state_source=$(findmnt -n -o SOURCE /run/initramfs/cos-state)
state_part=$(readlink -f "$state_source")
boot_parent=$(lsblk -ndo PKNAME "$state_part")
test -n "$boot_parent" || {
  echo "Cannot determine the COS_STATE parent disk" >&2
  exit 1
}
boot_disk="/dev/$boot_parent"

test "$persist_disk" != "$boot_disk" || {
  echo "Refusing to resize persistence on boot disk $boot_disk" >&2
  exit 1
}

partition_number=$(lsblk -ndo PARTN "$persist_part")
test "$partition_number" = 1 || {
  echo "Expected COS_PERSISTENT to be partition 1, found $partition_number" >&2
  exit 1
}

printf 'persistent partition: %s\n' "$persist_part"
printf 'persistent disk:      %s\n' "$persist_disk"
printf 'boot disk:            %s\n' "$boot_disk"

findmnt /usr/local
lsblk -o NAME,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINTS "$persist_disk" "$boot_disk"
sudo parted -s "$persist_disk" unit GiB print free
```

Continue only when all of the following are true:

- `/usr/local` is mounted from the `COS_PERSISTENT` partition.
- The persistent disk and boot disk are different devices.
- `COS_PERSISTENT` is partition 1 and the only partition on its disk.
- The disk is larger than the partition and the unallocated space follows
  partition 1.

## Grow the Partition and Filesystem

Reduce writes while changing the partition table:

```sh
if systemctl is-active --quiet k3s-agent; then
  sudo systemctl stop k3s-agent
elif systemctl is-active --quiet k3s; then
  sudo systemctl stop k3s
fi
```

Extend the GPT partition to the end of the disk. `--fix` moves the backup GPT
header when the virtual disk has grown:

```sh
sudo parted --script --fix "$persist_disk" \
  resizepart "$partition_number" 100%
sudo partprobe "$persist_disk"
sudo udevadm settle
```

Check whether the kernel sees the new partition size:

```sh
lsblk -b -o NAME,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINTS "$persist_disk"
```

If `partprobe` reports that the device is busy, or the partition still has its
old size, reboot the drained node. Re-identify the devices and confirm the
partition size before continuing. Do not run `resize2fs` against a partition
whose size has not changed.

Once the larger partition is visible, grow ext4 online:

```sh
sudo resize2fs /dev/disk/by-label/COS_PERSISTENT
df -h /usr/local
lsblk -o NAME,SIZE,FSTYPE,LABEL,MOUNTPOINTS "$persist_disk"
```

## Return the Node to Service

If the node was not rebooted, restart its K3s service:

```sh
if systemctl list-unit-files k3s-agent.service --no-legend | grep -q k3s-agent; then
  sudo systemctl start k3s-agent
else
  sudo systemctl start k3s
fi
```

From the operator machine, wait for the node to become Ready and then uncordon
it:

```sh
kubectl --kubeconfig="$HOME/.kube/k2/k2/kubeconfig" get nodes
kubectl --kubeconfig="$HOME/.kube/k2/k2/kubeconfig" uncordon <node-name>
```

Finally, verify the persistence marker and mount on the node:

```sh
findmnt /usr/local
test -f /usr/local/.state/.k2-persistent-ok
```
