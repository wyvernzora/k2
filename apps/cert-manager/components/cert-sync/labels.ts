export const CERT_SYNC_APP_NAME = "cert-sync";
export const CERT_SYNC_PROXMOX_COMPONENT = "proxmox";
export const CERT_SYNC_TRUENAS_COMPONENT = "truenas";

export const CERT_SYNC_PROXMOX_LABELS = {
  "app.kubernetes.io/name": CERT_SYNC_APP_NAME,
  "app.kubernetes.io/component": CERT_SYNC_PROXMOX_COMPONENT,
};

export const CERT_SYNC_TRUENAS_LABELS = {
  "app.kubernetes.io/name": CERT_SYNC_APP_NAME,
  "app.kubernetes.io/component": CERT_SYNC_TRUENAS_COMPONENT,
};
