# K2 Tools

`k2-tools` is the client-side toolbox for clean K2 Kairos images.
It supports bootstrap-server, additional server, and worker provisioning over
SSH for the raw image path, local QEMU VM management, and CM4 / ComputeBlade
eMMC flashing.

It assumes the target node has booted into the installed Kairos system, SSH is
reachable, k3s is installed but disabled, and the image contains the invariant
server config under `/usr/share/k2/node-provision/k3s/`.

## Build And Test

Build the local development binary before running toolbox commands:

```sh
cd tools
go test ./...
go vet ./...
go build -o k2-tools ./cmd/k2-tools
./k2-tools --help
cd ..
```

Install the local CLI into your Go bin directory:

```sh
cd tools
go install ./cmd/k2-tools
```

## Render Bootstrap Files

Use render mode to inspect the exact files before touching a node:

```sh
./tools/k2-tools provision render bootstrap \
  --cluster-target v3 \
  --cluster-name v3-test \
  --node-name v3-test-01 \
  --bootstrap-api-host 10.0.2.15 \
  --operator-key-file ~/.ssh/id_ed25519.pub \
  --extra-manifests '/path/to/bootstrap/*.yaml' \
  --output-dir /tmp/k2-tools-render
```

`--cluster-target` selects the source cluster config and generated deploy tree,
for example `clusters/v3.yaml` and `deploy/`. `--cluster-name` is the local
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

- `K2_TOOLS_REPO_ROOT`
- `K2_TOOLS_PLAIN`
- `K2_PROVISION_CLUSTER_TARGET`
- `K2_PROVISION_CLUSTER_NAME`
- `K2_PROVISION_NODE_NAME`
- `K2_PROVISION_HOST`
- `K2_PROVISION_SSH_PORT`
- `K2_PROVISION_SSH_USER`
- `K2_PROVISION_TEST_VM`
- `K2_PROVISION_OPERATOR_KEY`
- `K2_PROVISION_OPERATOR_KEY_FILE`
- `K2_PROVISION_LABEL`
- `K2_PROVISION_TAINT`
- `K2_PROVISION_SERVER_URL`
- `K2_PROVISION_EXTRA_MANIFESTS`
- `K2_PROVISION_OUTPUT_DIR`

Command-line flags still win over environment values, so per-node fields such
as `--host`, `--ssh-port`, and `--node-name` can stay explicit.

The default output is grouped status output. Set `K2_TOOLS_PLAIN=1` or pass
`--plain` when you need the older line-oriented `k2-tools:` output for
logs or scripts.

## Local QEMU VMs

`./tools/k2-tools vm` manages local QEMU VMs for development and validation. The
default `qemu-user` preset uses
the host-architecture QEMU artifact, a second persistent disk, user-mode
networking, SSH forwarding, Kubernetes API forwarding, a QEMU monitor port, and
QEMU guest-agent wiring.

Use `qemu-vmnet` for multi-node cluster tests. It uses macOS `vmnet-shared`
networking so VMs share a subnet and keep outbound connectivity. macOS requires
QEMU to have vmnet privileges for this backend; use `--sudo` for local test
VMs. Do not ad-hoc sign QEMU with the `com.apple.vm.networking` entitlement:
macOS treats that as a restricted entitlement and kills the binary before it
can start.

```sh
./tools/k2-tools vm create --id v3a --start
./tools/k2-tools vm create qemu-vmnet --id v3b --start
./tools/k2-tools vm start --sudo v3b
./tools/k2-tools vm status v3a
./tools/k2-tools vm console v3a
./tools/k2-tools vm ssh v3a
```

VM runtime state lives under `.testvm/vm-<id>/` and remains ignored by
git. `vm create` prefers locally built `kairos/artifacts/<target>/*.raw.xz`
files. If no local artifact exists, it reads the public S3 latest manifest,
downloads the matching artifact, verifies its SHA256, and caches it under
`.testvm/cache/artifacts/`.

Useful VM commands:

- `./tools/k2-tools vm presets`
- `./tools/k2-tools vm list`
- `./tools/k2-tools vm info <id>`
- `./tools/k2-tools vm start <id>`
- `./tools/k2-tools vm stop <id>`
- `./tools/k2-tools vm stop --all`
- `./tools/k2-tools vm delete --force <id>`
- `./tools/k2-tools vm delete --all --force`

`vm list` prints the best guest IP reported by the QEMU guest agent, using the
same address selection as `vm ssh`. It prints `-` while a VM is stopped or while
the guest agent has not reported an address yet.

## Provision Bootstrap Node

Bootstrap provisioning connects to the node with the built-in Go SSH transport
and uploads files with SFTP.

```sh
./tools/k2-tools provision bootstrap \
  --cluster-target v3 \
  --cluster-name v3-test \
  --host 127.0.0.1 \
  --ssh-port 2222 \
  --node-name v3-test-01 \
  --operator-key-file ~/.ssh/id_ed25519.pub \
  --extra-manifests '/path/to/bootstrap/*.yaml'
```

`--extra-manifests` accepts a manifest path or a glob and can be repeated. The
matched files are appended to the bootstrap bundle verbatim, after the minimum
Cilium, Argo CD, and kube-vip apps. The root Argo CD app-of-apps is staged
separately and applied after the Argo Application CRD is established; it does
not auto-sync, while generated child Applications use the cluster config's
auto-sync setting. Quote globs when you want `k2-tools` to expand them
deterministically instead of letting the shell expand them first.

When an extra manifest uses `metadata.namespace`, the provisioner prepends a
minimal generated Namespace manifest unless that namespace is already part of
the minimum bundle. The extra manifest still applies verbatim afterward, so a
user-supplied Namespace manifest can add labels or annotations. This makes a
bootstrap Secret as simple as:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: bootstrap-secret
  namespace: secrets
type: Opaque
stringData:
  provider-token: "..."
```

For local VM swarm tests, use `--test-vm <id>` instead of `--host`. The
provisioner resolves the VM SSH endpoint, defaults `--cluster-name` to
`<cluster-target>-vmtest`, defaults `--node-name` to the VM id, uses the guest
IP for the bootstrap-only Cilium API patch, adds a VM-local API VIP to the
bootstrap server TLS SANs, and patches the kube-vip DaemonSet after bootstrap
so the saved kubeconfig and join URL point at an address in the VM subnet:

```sh
./tools/k2-tools provision bootstrap \
  --cluster-target v3 \
  --test-vm v3a \
  --operator-key-file ~/.ssh/id_ed25519.pub
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
`clusters/<target>.yaml`. For `--test-vm` bootstrap, the saved endpoint is the
VM-subnet kube-vip address chosen during provisioning.

After credentials are harvested, the provisioner SSHes back into the node and
verifies the expected post-reboot state: hostname, operator SSH keys, active
K3s config files, disabled stock Kairos credentials, disabled packaged
manifests, enabled and active `k3s`, and generated K3s credentials. The command
fails if those checks do not converge.

SSH authentication is selected once at the start and reused for the rest of the
run. The provisioner first tries the clean Kairos default password `kairos`,
then tries the local SSH agent and unencrypted default private keys, then
prompts for a password and caches it for the rest of the run. Loopback targets
skip host-key checks for VM port-forward convenience; non-loopback targets use
`~/.ssh/known_hosts` with accept-new behavior for first contact.

The bootstrap manifest bundle is intentionally small: Cilium, Argo CD,
kube-vip, namespace manifests, optional extra manifests, and the Argo CD root
Application. Normal GitOps sync is expected to take over after Argo CD is
running.

## Provision Additional Nodes

Additional server and worker provisioning uses the local operator state written
by bootstrap provisioning under `~/.kube/k2/<cluster-name>/`.

Servers use `server-token`, activate the baked server invariant config, and get
the automatic control-plane taint:

```sh
./tools/k2-tools provision server \
  --cluster-target v3 \
  --cluster-name v3-test \
  --host 10.42.0.23 \
  --node-name v3-test-02 \
  --operator-key-file ~/.ssh/id_ed25519.pub
```

For VM swarm tests, `--test-vm` also defaults the cluster name to
`<cluster-target>-vmtest`, defaults the node name to the VM id, and uses the VM
SSH endpoint:

```sh
./tools/k2-tools provision server --cluster-target v3 --test-vm v3b
./tools/k2-tools provision worker --cluster-target v3 --test-vm v3c
```

Workers use `agent-token`, enable `k3s-agent`, and do not activate server-only
invariants:

```sh
./tools/k2-tools provision worker \
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

## Flash CM4 / ComputeBlade eMMC

`./tools/k2-tools flash rpi4cb` writes a built `*-arm64-rpi4cb-k8s` artifact to the
CM4 eMMC exposed through `rpiboot`. Build the artifact first
(`earthly --allow-privileged ./kairos+image-build-artifact
--KAIROS_TARGET=ubuntu-26.04-arm64-rpi4cb-k8s`), then with the
CM4 disconnected:

```sh
./tools/k2-tools flash rpi4cb --zero-nvme
```

The flasher snapshots existing external disks, runs `rpiboot` (asking
you to hold nRPIBOOT and connect USB-C), waits for the new disks to
appear, classifies them by size (≈32 GB → eMMC, ≈256 GB → NVMe), shows
a plan, and waits for a `FLASH` confirmation. After write it re-reads
the eMMC and confirms the SHA256 matches the artifact manifest. `--yes`
skips the prompt; `--skip-rpiboot` reuses currently-attached disks;
`--skip-verify` omits the readback. The macOS path needs `rpiboot`,
`diskutil`, `dd`, and `xz` on `PATH` — install rpiboot via
`brew install rpiboot` (and `brew install xz` if it's not already
there) or build rpiboot from
[raspberrypi/usbboot](https://github.com/raspberrypi/usbboot). Linux is
not yet supported; the Platform abstraction is built so a future Linux
implementation can plug in alongside `platform_darwin.go`.

## Current Limits

- raw-image reboot path only; live ISO `manual-install` is deferred
- local SSH config host aliases and encrypted key passphrases are not parsed by
  the built-in SSH transport
- `./tools/k2-tools flash rpi4cb` is macOS-only today; Linux support pending
