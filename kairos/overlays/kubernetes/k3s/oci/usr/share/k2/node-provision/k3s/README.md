# K2 K3s Node Provisioning Overlay

This directory is baked into the immutable Kairos image as the K2 K3s
provisioning contract area.

The overlay installs only invariant K2 K3s server configuration under
`/etc/rancher/k3s`. Active cluster-specific K3s configuration is written at
provision time by `k2-node-provision` from a bundle or node seed derived from
`clusters/<name>.yaml`.

Cluster-owned settings include:

- API VIPs, DNS names, and TLS SANs
- pod and service CIDRs
- cluster DNS and cluster domain
- bootstrap versus join intent
- token or token-file strategy
- node labels and taints
- bootstrap manifests

The baked image should stay reusable across K2 clusters and nodes.
