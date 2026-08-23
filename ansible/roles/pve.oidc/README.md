# pve.oidc

Configures the post-K2 Pocket ID realm on a Proxmox VE host. This role is intentionally absent from `pve-bootstrap.yml`: the PVE host and its PAM administrator must remain usable before K2 and Pocket ID exist.

Pocket ID setup creates the `proxmox-admins` group, restricts the `proxmox-shuna` client to that group, and writes the client credential to the `pocket-id/proxmox-shuna-oidc` Secret. Export that Secret without decoding it:

```sh
install -d -m 0700 ansible/credentials/oidc
umask 077
KUBECONFIG="$HOME/.kube/k2/k2/kubeconfig" kubectl -n pocket-id get secret proxmox-shuna-oidc -o json > ansible/credentials/oidc/shuna.json
```

Then run the separate playbook:

```sh
earthly ./ansible+image
docker run --rm \
  -v "$PWD/ansible/inventory:/ansible/inventory:ro" \
  -v "$PWD/ansible/credentials:/ansible/credentials:ro" \
  -v "$HOME/.ssh:/root/.ssh:ro" \
  k2-ansible:local pve-oidc --limit shuna
```

The realm is not made the default. `wyvernzora@pam` remains the independent break-glass account, and the playbook refuses to proceed unless its live ACL is still exactly `Administrator` on `/` with propagation. The resulting `wyvernzora@k2` ACL must match it exactly.
