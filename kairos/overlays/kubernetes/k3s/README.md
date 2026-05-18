# K2 K3s Overlay

This overlay is the invariant K3s capability layer for K2 Kairos nodes.

It intentionally does not bake active K3s configuration into
`/etc/rancher/k3s`. Cluster-specific values such as API VIPs, TLS SANs, pod and
service CIDRs, cluster DNS, bootstrap intent, join endpoints, tokens, labels,
taints, and bootstrap manifests are supplied by a bundle or node seed and
applied later by `k2-node-provision`.

Keep this overlay limited to assets that are safe for every K2 K3s node image:
provisioner contracts, schemas, examples, documentation, and eventually the
cluster-agnostic provisioner binary or service glue.
