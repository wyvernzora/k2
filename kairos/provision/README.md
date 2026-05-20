# K2 Node Provision

`k2-provision` is the client-side Kairos node provisioner for clean K2 images.
It supports bootstrap-server, additional server, and worker provisioning over
SSH for the raw image path.

It assumes the target node has booted into the installed Kairos system, SSH is
reachable, k3s is installed but disabled, and the image contains the invariant
server config under `/usr/share/k2/node-provision/k3s/`.

## Build And Test

```sh
cd kairos/provision
go test ./...
go vet ./...
go run ./cmd/k2-provision --help
```

## Render Bootstrap Files

Use render mode to inspect the exact files before touching a node:

```sh
cd kairos/provision
go run ./cmd/k2-provision render bootstrap \
  --cluster-target v3 \
  --cluster-name v3-test \
  --node-name v3-test-01 \
  --bootstrap-api-host 10.0.2.15 \
  --operator-key-file ~/.ssh/id_ed25519.pub \
  --onepassword-token-file /path/to/onepassword-service-account-token \
  --output-dir /tmp/k2-provision-render
```

`--cluster-target` selects the source cluster config and generated deploy tree,
for example `clusters/v3.yaml` and `deploy/v3`. `--cluster-name` is the local
cluster instance name and is reserved for per-test credential state such as
`~/.kube/k2/<cluster-name>/`.

Local render mode does not include baked image metadata labels because those
come from `/usr/share/k2/image-build/metadata.yaml` on the target node during
real provisioning.

`--bootstrap-api-host` patches the bootstrap-only Cilium manifest so Cilium can
reach the Kubernetes API before kube-vip advertises the cluster VIP. Real SSH
provisioning auto-detects this from the target node when the flag is omitted;
render mode needs the value passed explicitly when you want to inspect that
patch.

Operator keys must be literal `ssh-ed25519` public keys. `github:` key sources
are intentionally rejected; callers can fetch and review those keys before
passing them in.

## Common Environment Defaults

Most flags can be supplied through environment variables, which keeps repeated
test provisioning commands short. For example:

```sh
export K2_PROVISION_CLUSTER_TARGET=v3
export K2_PROVISION_CLUSTER_NAME=v3-test
export K2_PROVISION_OPERATOR_KEY_FILE=$HOME/.ssh/id_ed25519.pub
export K2_PROVISION_SSH_USER=kairos
```

Useful variables include:

- `K2_PROVISION_REPO_ROOT`
- `K2_PROVISION_CLUSTER_TARGET`
- `K2_PROVISION_CLUSTER_NAME`
- `K2_PROVISION_NODE_NAME`
- `K2_PROVISION_HOST`
- `K2_PROVISION_SSH_PORT`
- `K2_PROVISION_SSH_USER`
- `K2_PROVISION_OPERATOR_KEY`
- `K2_PROVISION_OPERATOR_KEY_FILE`
- `K2_PROVISION_LABEL`
- `K2_PROVISION_TAINT`
- `K2_PROVISION_SERVER_URL`
- `K2_PROVISION_ONEPASSWORD_TOKEN_FILE`
- `K2_PROVISION_OUTPUT_DIR`

Command-line flags still win over environment values, so per-node fields such
as `--host`, `--ssh-port`, and `--node-name` can stay explicit.

## Provision Bootstrap Node

Bootstrap provisioning shells out to local OpenSSH tools. The CLI checks for
`ssh` and `scp` on startup and fails before touching the node if either is
missing.

```sh
cd kairos/provision
go run ./cmd/k2-provision bootstrap \
  --cluster-target v3 \
  --cluster-name v3-test \
  --host 127.0.0.1 \
  --ssh-port 2222 \
  --node-name v3-test-01 \
  --operator-key-file ~/.ssh/id_ed25519.pub \
  --onepassword-token-file /path/to/onepassword-service-account-token
```

The command writes:

- active server invariant config copied from `/usr/share/k2`
- cluster-wide K3s config from `clusters/<target>.yaml`
- bootstrap server role config with control-plane labels and taints
- Kairos cloud-config enabling the k3s provider
- the minimum bootstrap manifest bundle under the k3s auto-deploy directory
- operator SSH authorized keys for the `kairos` user

During bootstrap, the rendered Cilium resources use the bootstrap node IP as
`KUBERNETES_SERVICE_HOST`. The steady-state deploy manifests still use the
cluster API VIP from `clusters/<target>.yaml`, so once kube-vip and Argo CD are
running GitOps reconciles Cilium back to the normal cluster-wide endpoint.

By default the target node reboots after files are installed. Pass
`--no-reboot` to inspect the target before restart.

After reboot, the provisioner waits for SSH and k3s credentials, then writes
local operator state under `~/.kube/k2/<cluster-name>/`:

- `kubeconfig`
- `server-token`
- `node-token`
- `agent-token`
- `server-url`

The saved kubeconfig has its server URL rewritten from the node-local
`https://127.0.0.1:6443` endpoint to the API endpoint from
`clusters/<target>.yaml`.

After credentials are harvested, the provisioner SSHes back into the node and
verifies the expected post-reboot state: hostname, operator SSH keys, active
K3s config files, disabled stock Kairos credentials, disabled packaged
manifests, enabled and active `k3s`, and generated K3s credentials. The command
fails if those checks do not converge.

SSH authentication is selected once at the start and reused for all SSH/SCP
calls. The provisioner first tries the clean Kairos default password
`kairos`, then tries local SSH config in batch mode, then prompts for a
password and caches it for the rest of the run.

The bootstrap manifest bundle is intentionally small: Cilium, Argo CD,
kube-vip, namespace manifests, an optional 1Password service-account Secret,
and the Argo CD root Application. Normal GitOps sync is expected to take over
after Argo CD is running.

## Provision Additional Nodes

Additional server and worker provisioning uses the local operator state written
by bootstrap provisioning under `~/.kube/k2/<cluster-name>/`.

Servers use `server-token`, activate the baked server invariant config, and get
the automatic control-plane taint:

```sh
cd kairos/provision
go run ./cmd/k2-provision server \
  --cluster-target v3 \
  --cluster-name v3-test \
  --host 10.42.0.23 \
  --node-name v3-test-02 \
  --operator-key-file ~/.ssh/id_ed25519.pub
```

Workers use `agent-token`, enable `k3s-agent`, and do not activate server-only
invariants:

```sh
cd kairos/provision
go run ./cmd/k2-provision worker \
  --cluster-target v3 \
  --cluster-name v3-test \
  --host 10.42.0.31 \
  --node-name v3-test-worker-01 \
  --operator-key-file ~/.ssh/id_ed25519.pub
```

Both commands default the join endpoint from
`~/.kube/k2/<cluster-name>/server-url`, falling back to the API VIP from
`clusters/<target>.yaml`. The saved join URL intentionally uses the VIP IP, not
the API DNS name, because cluster DNS depends on Kubernetes already being
healthy. Use `--server-url https://<host>:6443` when a test network needs nodes
to join through a bootstrap node address rather than the cluster API VIP.

After reboot, server and worker provisioning SSH back into the node and verify
role-specific state. Servers must have the invariant server config, cluster
config, packaged-manifest skip files, and active `k3s`. Workers must have only
worker join config, no server-only invariant or cluster config, and active
`k3s-agent`.

## Current Limits

- raw-image reboot path only; live ISO `manual-install` is deferred
- system `ssh` and `scp` are used for transport
