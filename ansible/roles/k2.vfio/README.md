# k2.vfio

Configures IOMMU and early per-device VFIO binding on Proxmox VE hosts.

Hosts must belong to exactly one CPU group:

- `proxmox_intel` enables `intel_iommu=on`
- `proxmox_amd` relies on firmware-enabled AMD IOMMU discovery; current kernels
  reject the legacy `amd_iommu=on` option

The role supports both GRUB hosts and ZFS-root UEFI hosts managed by
`proxmox-boot-tool`. It blacklists local GPU and HDMI-audio drivers, loads VFIO
modules in initramfs, and renders exact PCI addresses from `vfio_devices` into
an initramfs binding script. An empty device list enables IOMMU without binding
any devices. Ignoring unhandled guest MSRs is disabled by default; set
`k2_vfio_ignore_msrs: true` only for a guest that demonstrably requires it.

A reboot is required after kernel-command-line or initramfs changes. Verify the
result with `dmesg`, `lspci -Dnnk`, and the generated
`/etc/initramfs-tools/scripts/init-top/bind_vfio` before assigning devices to a
VM.
