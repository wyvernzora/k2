# Kairos Runbooks

This directory contains operator runbooks for common K2 Kairos node scenarios.
These are maintenance procedures for deployed nodes, not image-build or
provisioning design documents.

## Storage

- [Expand a dedicated persistent disk](expand-persistent-disk.md) — grow the
  single ext4 `COS_PERSISTENT` partition after enlarging a virtual disk.
- [Migrate a legacy boot layout](migrate-legacy-boot-layout.md) — remove the
  obsolete `COS_PERSIST_OLD` partition and grow `COS_STATE` from a recovery
  boot, using label-driven discovery for both QEMU and Raspberry Pi nodes.

Runbooks favor explicit device discovery and verification over remembered
device names. Stop when observed devices, labels, mounts, or partition layouts
do not match the stated prerequisites.
