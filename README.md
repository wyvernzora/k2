<div align="center">
    <br>
    <br>
    <img width="182" src=".assets/k2.png">
    <h1 align="center">K2 Ansible Playbooks</h1>
</div>

<p align="center">
<b>Ansible playbooks for provisioning various bits of K2, my homelab cluster.</b>
</p>

<hr>

## Why containers?
Repeatability. I have previously encountered issues setting up Ansible on my local machine, then making sure that its version is just right, then gathering all the dependencies and making sure their versions are just right. With containers everything is bundled in. If it works now, it will continue working down the line, just a `docker run` away.

## K2 Commons
These are bits and pieces that are not specific to any environment, and are applicable across different pieces of K2 infrastructure.

### `k2.fish`
Installs fish shell for specified users and sets up config and plugins just the way I like them.

### `k2.tls`
Pulls the latest TLS certificates from my S3 bucket and uploads them to the following directories on remote host:
 - Certificate: `/etc/ssl/certs/{{ domain }}.pem`
 - Private Key: `/etc/ssl/private/{{ domain }}.pem`

### `k2.user`
Creates a non-admin user, sets up SSH keys and grants it passwordless sudo access.

### `k2.vfio`
Sets up all the configs to bind PCI devices to VFIO driver and pass them through to VMs.

## [ProxmoxVE](playbooks/proxmox/README.md)
Published as `ghcr.io/wyvernzora/k2-ansible-proxmox`

 - Replaces enterprise APT repos with no-subscription repos
 - Suppresses the "no-subscription" popup on login
 - Creates non-admin user via `k2.user` and adds as admin to the PVE cluster
 - Uploads TLS cert using `k2.tls` and sets up NGINX for TLS termination of admin UI
 - Sets up fish shell for `root` and non-admin user via `k2.fish`
 - Sets up PCI passthrough via `k2.vfio`
