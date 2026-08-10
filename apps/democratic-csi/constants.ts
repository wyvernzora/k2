/**
 * Storage appliance facts (k2-st-0e12, shuna VM 220). These are addresses
 * and names, not secrets — the CSI SSH private key and CHAP credentials live
 * in 1Password and reach the cluster only through ExternalSecrets.
 */
export const APPLIANCE_FABRIC_ADDRESS = "172.16.9.250";
export const APPLIANCE_SSH_PORT = 22;

export const CSI_DRIVER_NAME = "org.democratic-csi.iscsi";

export const DRIVER_CONFIG_SECRET_NAME = "democratic-csi-driver-config";
export const NODE_STAGE_SECRET_NAME = "democratic-csi-node-stage";
