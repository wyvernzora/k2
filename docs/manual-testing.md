# Manual testing scenarios

Hands-on validation recipes for Kairos images, node provisioning, the
storage appliance, and the e2e harness. Everything here runs from a macOS
host with `k2-tools` installed (`go install ./cmd/k2-tools` from `tools/`
— reinstall after every change to `tools/`, the shim resolves to
`~/go/bin`).

Conventions used throughout:

- `K2_TOOLS_VM_SUDO_QEMU=1` is required for any multi-VM scenario: vmnet
  networking needs QEMU launched through sudo. The tools pre-authorize
  sudo up front; you get one password prompt at the start of a run.
- Test-VM state lives in `.testvm/` (repo root). Per-cluster credentials
  (kubeconfig, tokens, operator key, `storage-appliance.json`) live in
  `~/.kube/k2/<cluster-name>/`.
- All SSH against test VMs uses relaxed host keys; production nodes never
  do.

## 0. Prerequisites

```sh
brew install qemu xz helm kubectl
cd tools && go install ./cmd/k2-tools
k2-tools image plan            # sanity: targets resolve
k2-tools vm presets            # qemu-user | qemu-vmnet | qemu-vmnet-storage
```

Artifacts must exist locally before VM tests:

```sh
k2-tools image build artifact ubuntu-26.04-arm64-qemu-k8s
k2-tools image build artifact ubuntu-26.04-arm64-qemu-storage
```

## 1. Single-VM image smoke test

Boots an image, verifies role metadata and first-boot behavior
(recovery → auto-reset → active).

```sh
k2-tools vm create qemu-user --id smoke --start
k2-tools vm status smoke            # wait for guest_ipv4 to appear
k2-tools vm console smoke           # watch first boot; Ctrl-C detaches
k2-tools vm ssh smoke               # user kairos, password kairos
  cat /usr/share/k2/image-build/metadata.yaml   # role/arch/hardware must match target
k2-tools vm delete smoke --force
```

What to check on a storage image (`vm create qemu-vmnet-storage --sudo`):
`zfs version`, `targetcli ls`, `k2-node-agent storage-health` (expect
UNHEALTHY "no ZFS pools imported" on an unprovisioned appliance — that is
by design), and that `systemctl status k2-zfs-load-key` shows the unit
present but conditioned out of recovery boots.

## 2. Storage appliance: provision + reboot + DR drill

The full appliance lifecycle by hand. Uses `--test-vm`, which resolves
host/cluster-name automatically and relaxes SSH policy.

```sh
export K2_TOOLS_VM_SUDO_QEMU=1
k2-tools vm create qemu-vmnet-storage --id stor1 --start --sudo
k2-tools vm status stor1                       # note the guest IP

k2-tools provision storage \
  --cluster-target v3 --test-vm stor1 \
  --pool-vdev "mirror /dev/vdc /dev/vdd"
```

Walk the plan output (pool topology, wipe verdicts per disk), confirm,
then verify:

- Banner reports the credentials file; inspect
  `~/.kube/k2/v3-vmtest/storage-appliance.json` — portal, IQN base, csi
  key, CHAP, and the escrowed `poolKey`.
- On the appliance (`k2-tools vm ssh stor1`): `sudo zpool status`,
  `sudo zfs get keystatus tank` → `available`,
  `getent passwd csi`, `sudo cat /oem/95-k2-storage-csi.yaml`.

**Reboot drill** (validates the D26 boot chain and csi-user persistence —
both have regressed before):

```sh
# from an ssh session on the appliance: sudo reboot
# wait for it to come back, then:
#   keystatus must be 'available' WITHOUT any manual intervention
#   getent passwd csi must still resolve
#   sudo k2-node-agent storage-health → healthy
```

**DR drill** (credential reuse across a reset):

```sh
# on the appliance: sudo kairos-agent reset  (wipes persistent partition)
# after it boots back into a clean active system:
k2-tools provision storage --cluster-target v3 --test-vm stor1 \
  --pool-vdev "mirror /dev/vdc /dev/vdd"
# must REUSE the existing storage-appliance.json (log line says so),
# restore the same pool key, and import the existing pool without
# --force-wipe. The pool key is never regenerated while the file exists.
```

Cleanup: `k2-tools vm delete stor1 --force` and remove
`~/.kube/k2/v3-vmtest/` if you're done with the credentials.

## 3. k8s cluster by hand (bootstrap + worker)

```sh
export K2_TOOLS_VM_SUDO_QEMU=1
k2-tools vm create qemu-vmnet --id srv1 --start --sudo
k2-tools vm create qemu-vmnet --id wrk1 --start --sudo

k2-tools provision bootstrap --cluster-target v3 --test-vm srv1
k2-tools provision worker    --cluster-target v3 --test-vm wrk1

export KUBECONFIG=~/.kube/k2/v3-vmtest/kubeconfig
kubectl get nodes    # both Ready; API answers on the derived test VIP
```

Notes: the kube-vip test VIP is substituted into the rendered manifests
at bundle time (last-usable address in the VM subnet); the root Argo CD
app is applied but has no automated sync — it must sit `OutOfSync` and
deploy nothing. Worker join can legitimately take up to ~10 minutes on
first boot (image pulls gate the API VIP).

## 4. Full e2e scenarios (the preferred path)

Everything in §2–§3 plus CSI/PVC validation, as one command:

```sh
k2-tools e2e list
K2_TOOLS_VM_SUDO_QEMU=1 k2-tools e2e run storage-pvc --skip-teardown-on-fail
K2_TOOLS_VM_SUDO_QEMU=1 k2-tools e2e run k8s-wireup  --skip-teardown-on-fail
```

A clean `storage-pvc` run takes ~8 minutes from nothing and covers:
provisioning the encrypted appliance, an appliance reboot (D26 chain +
csi-user persistence), bootstrap with the rendered test VIP, worker join,
democratic-csi install (gated on rollout), PVC bind + pod IO,
ZFS/LIO consistency as the csi user, and delete hygiene. On success it
tears down VMs, credentials, and scratch.

Flags that matter:

- `--skip-teardown-on-fail` — keep everything on failure for post-mortem.
  Reruns reuse the preserved VMs and credentials; the operator key
  persists in `~/.kube/k2/k2e2e/` precisely so reruns can authenticate
  to already-hardened nodes.
- `--keep` — keep everything even on success.
- `k2-tools e2e storage` is an alias for `e2e run storage-pvc`.

## 5. Troubleshooting a failed run

- **See what a VM is doing:** `k2-tools vm console <id>` (serial),
  `k2-tools vm status <id>`, `k2-tools vm list`.
- **SSH into e2e nodes:** as `kairos` with
  `-i ~/.kube/k2/k2e2e/operator_ed25519` post-provision (password auth is
  disabled by hardening); fresh nodes accept kairos/kairos.
- **Storage appliance sshd penalty box:** repeated failed auth (e.g. a
  crash-looping CSI pod) makes OpenSSH refuse connections from that
  source for minutes (`PerSourcePenalties`). It looks like
  ECONNREFUSED. Check the appliance journal for `Invalid user` /
  `Disconnected` lines before blaming the network or CNI.
- **Wedged/orphaned QEMU:** `ps aux | grep qemu-system`. Duplicate
  processes per VM name or processes surviving `vm delete` are bugs —
  `vm delete --all --force` now finds root QEMUs via pgrep and escalates
  powerdown → TERM → KILL through sudo.
- **Full reset:** `k2-tools vm delete --all --force`, then remove
  `~/.kube/k2/k2e2e/`. Never delete the credentials dir while hardened
  VMs are still running — the operator key inside is the only way back
  into them.

## 6. Physical-hardware paths (not VM-testable)

- **rpi4cb flash:** `k2-tools flash rpi4cb` onto a CM4. Watch item for
  the first 26.04 flash: verify the "Add state partition" layout stage
  in the boot journal before trusting the node — this path cannot be
  exercised in QEMU.
- **In-place upgrade:** `k2-tools upgrade --cluster-name <name> --host
  <ip> [--identity <key>]` — hardened nodes need `--identity` (or an
  agent-loaded key). Verifies the active image post-reboot.
