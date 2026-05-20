# K2 K3s Overlay

This overlay is the invariant K3s layer for K2 Kairos nodes.

It bakes the invariant K2 server config that all K2 K3s server nodes should
share under `/usr/share/k2/node-provision/k3s/`. The file is not active in the
base image. Node provisioning copies it into `/etc/rancher/k3s/config.yaml.d/`
only when the target node is a K3s server.

The overlay does not enable the K3s service. `k2-tools provision` writes the
role activation cloud-config after role-specific config and bootstrap manifests
are in place.

Cluster-specific values such as API VIPs, TLS SANs, pod and service CIDRs,
cluster DNS, bootstrap intent, join endpoints, tokens, labels, taints, and
bootstrap manifests are supplied by the provisioner.

Keep this overlay limited to files that are safe to bake into every K2 K3s
image. Active server-only config must stay inert until provisioning chooses a
server role.
