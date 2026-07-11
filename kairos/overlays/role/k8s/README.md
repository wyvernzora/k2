# K2 k8s Role Overlay

Kubernetes node role overlay for K2 Kairos images.

It bakes invariant K3s config under `/usr/share/k2/node-provision/k3s/`,
time-sync service ordering, the unified persistent-storage cloud-config, and
a persistent node-unique iSCSI initiator identity with `iscsid` enabled. The
Dockerfile installs `k2-node-agent` plus the disk utilities and initrd contents
declared in `overlay.yaml`.

Kubernetes nodes require a dedicated non-boot `COS_PERSISTENT` disk. Their raw
boot artifacts create only `COS_STATE`; the role disables Kairos' generic
persistent-partition growth stage because K2 initializes the external disk once
at its final size.

The overlay does not enable K3s. `./tools/k2-tools provision` writes role
activation config and cluster-specific K3s settings at provision time.
