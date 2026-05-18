# K2 K3s Overlay

This overlay is the invariant K3s server layer for K2 Kairos nodes.

It bakes the K3s provider enablement and the invariant K2 server config that
all K2 K3s server nodes should share.

Cluster-specific values such as API VIPs, TLS SANs, pod and service CIDRs,
cluster DNS, bootstrap intent, join endpoints, tokens, labels, taints, and
bootstrap manifests are supplied by a bundle or node seed and applied later by
`k2-node-provision`.

Keep this overlay limited to values that are safe for every K2 K3s server image.
