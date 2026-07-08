# Storage Role Overlay

Kairos storage-appliance role overlay.

This role installs ZFS and LIO/targetcli packages through `overlay.yaml`, bakes
the storage service wiring, and runs health checks through
`/usr/sbin/k2-node-agent storage-health`. It must not include
`/system/oem/05-persistent-storage.yaml`; storage images keep persistent state
on the boot disk so passed-through pool disks are never auto-claimed.

Run the checks with:

```sh
./tools/k2-tools image inspect oci ubuntu-24.04-amd64-qemu-storage
./tools/k2-tools image inspect artifact ubuntu-24.04-amd64-qemu-storage
```
