# Base Overlay

Shared Kairos image content that every K2 target must carry.

This ships the active-boot `k2-rescue` deactivation config and replaces
AuroraBoot's raw-disk bootstrap config with a top-level `users:` declaration.
The latter keeps `kairos` password access available only until provisioning
installs operator keys, disables the bootstrap config, and locks the password.
Keep hardware behavior and role-specific services in their own overlays.
