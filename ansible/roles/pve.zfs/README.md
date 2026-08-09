# pve.zfs

Configures the maximum ZFS ARC size for Proxmox VE hosts, applies it to the
running kernel, and rebuilds initramfs when the persistent module configuration
changes. The default limit is 16 GiB.
