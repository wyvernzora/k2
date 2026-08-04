# pve.standalone

Restores normal persistent logging and removes cluster-only keepalived state on
standalone Proxmox VE hosts. It also undoes state previously applied by the
retired `pve.ssd` and `pve.vip` roles.
