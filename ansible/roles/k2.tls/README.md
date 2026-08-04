# k2.tls

Exposes the Proxmox VE management UI on standard HTTPS and prepares its
certificate-sync API identity.

The role installs nginx's stream module, redirects HTTP to HTTPS on the host's
management address, and passes TCP port 443 through to the native Proxmox UI on
port 8006. TLS remains terminated by `pveproxy`, so certificate uploads through
the supported PVE API automatically affect both ports without an nginx reload.

The role creates a privilege-separated `cert-sync@pve!k2-cert-sync` API token
with only `Sys.Modify` on the local node. PVE reveals the token secret only at
creation time, so Ansible writes it to:

`credentials/proxmox/<host>.json`

The credentials mount must be writable. The credential directory and JSON
file use modes `0700` and `0600`, and the ignored `credentials/` tree prevents
the secret from entering Git. If a token predates this role, its secret cannot
be recovered; keep the existing external credential or rotate the token.
