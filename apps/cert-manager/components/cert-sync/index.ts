import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";

export {
  CERT_SYNC_APP_NAME,
  CERT_SYNC_PROXMOX_COMPONENT,
  CERT_SYNC_PROXMOX_LABELS,
  CERT_SYNC_TRUENAS_COMPONENT,
  CERT_SYNC_TRUENAS_LABELS,
} from "./labels.js";
export { PROXMOX_PORT, TRUENAS_PORT, proxmoxHosts, truenasHost } from "./hosts.js";

import { ProxmoxCertSync } from "./proxmox/index.js";
import { TrueNasCertSync } from "./truenas/index.js";

export class CertSync extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new ProxmoxCertSync(this, "proxmox");
    new TrueNasCertSync(this, "truenas");
  }
}
