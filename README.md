<div align="center">
    <br>
    <br>
    <img width="182" src=".assets/k2.png">
    <h1 align="center">K2</h1>
</div>

<p align="center">
<b>IaC configuration for my homelab.</b>
</p>

<hr>
<br>
<br>

# Ansible

### Why containers?
Repeatability. I have previously encountered issues setting up Ansible on my local machine, then making sure that its version is just right, then gathering all the dependencies and making sure their versions are just right. With containers everything is bundled in. If it works now, it will continue working down the line, just a `docker run` away.

### Ansible Roles
| Role                                           | Description                                                   |
| ---------------------------------------------- | ------------------------------------------------------------- |
| [`k2.fish`](ansible/roles/k2.fish)             | Set up Fish shell just the way I like it                      |
| [`k2.tls`](ansible/roles/k2.tls/README.md)     | Pull the latest TLS certificates and update them where needed |
| [`k2.user`](ansible/roles/k2.user/README.md)   | Set up non-root user and permissions                          |
| [`k2.vfio`](ansible/roles/k2.vfio/README.md)   | Configure VFIO/IOMMU for PCI passthrough to VMs               |
| [`k2.user`](ansible/roles/pve.nosub/README.md) | Stop ProxmoxVE nagging about subscription                     |

### Ansible Playbooks

| Playbook        | Description                                       |
| --------------- | ------------------------------------------------- |
| `pve-bootstrap` | Set up ProxmoxVE hosts into a good starting state |
| `update-certs`  | Update TLS certificates everywhere                |
