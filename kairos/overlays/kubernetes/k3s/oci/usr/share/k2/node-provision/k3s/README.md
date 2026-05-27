# K2 K3s Node Provisioning Overlay

This directory is baked into the immutable Kairos image as the K2 K3s
provisioning contract area.

The overlay installs invariant K2 K3s server configuration as inert files in
this directory. The provisioner copies `10-k2-invariant.yaml` into
`/etc/rancher/k3s/config.yaml.d/` only for server roles.

Active cluster-specific K3s configuration is written at provision time by
`./tools/k2-tools provision` from `clusters/<target>.yaml` and the requested node role.

Cluster-owned settings include:

- API VIPs, DNS names, and TLS SANs
- pod and service CIDRs
- cluster DNS and cluster domain
- bootstrap versus join intent
- token or token-file strategy
- node labels and taints
- bootstrap manifests

The baked image should stay reusable across K2 clusters and nodes.
