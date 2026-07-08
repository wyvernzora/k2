# K2 k8s Role Overlay

Kubernetes node role overlay for K2 Kairos images.

It bakes invariant K3s config under `/usr/share/k2/node-provision/k3s/`,
time-sync service ordering, and the unified persistent-storage cloud-config.
The Dockerfile installs `k2-node-agent` plus the disk utilities and initrd
contents declared in `overlay.yaml`.

The overlay does not enable K3s. `./tools/k2-tools provision` writes role
activation config and cluster-specific K3s settings at provision time.
