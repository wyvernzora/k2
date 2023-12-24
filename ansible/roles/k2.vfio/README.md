<div align="center">
    <br>
    <br>
    <img width="182" src="../../../.assets/k2.png">
    <h1 align="center">k2.vfio</h1>
</div>

<p align="center">
<b>Configure VFIO/IOMMU for PCI passthrough to VMs</b>
</p>

<hr>
<br>
<br>

## What it does
 - Ensures that required kernel parameters are set
 - Blacklists all GPU drivers
 - Sets up some recommended configuration for NVIDIA GPUs
 - Creates an initramfs script to force specified devices into VFIO mode

## Notes
 - This role does not use the typical vendor-ID-based approach to forcing VFIO driver. Instead, it works on individual PCI devices.

