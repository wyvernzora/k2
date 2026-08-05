<div align="center">
    <br>
    <br>
    <img width="182" src="../../../.github/assets/k2.png">
    <h1 align="center">pve.nosub</h1>
</div>

<p align="center">
<b>Use the supported Proxmox VE no-subscription package channel.</b>
</p>

<hr>
<br>
<br>

## What it does
 - Supports the deb822 repository layout used by Proxmox VE 8 and 9
 - Disables the enterprise PVE repository
 - Removes the unused Ceph repository on these standalone hosts
 - Configures the official `pve-no-subscription` repository

The subscription notice is intentionally not patched. Its implementation is
version-sensitive and is not part of package repository management.
