import type { Construct } from "constructs";

import { ApexDomain, HelmCharts, K2Chart } from "@k2/cdk-lib";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

const LONGHORN_STORAGE_NODE_TAG = "k2-storage";
const LONGHORN_DASHBOARD_HOST_PREFIX = "longhorn";
const LONGHORN_FRONTEND_SERVICE_NAME = "longhorn-frontend";

export class Longhorn extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    HelmCharts.of(this).asChart(this, "longhorn", "longhorn", longhornValues());
    new AuthenticatedIngress(this, "dashboard-ingress", {
      host: ApexDomain.of(this).subdomain(LONGHORN_DASHBOARD_HOST_PREFIX),
      serviceName: LONGHORN_FRONTEND_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}

function longhornValues() {
  return {
    // Dashboard exposure is declared below as a Pomerium-authenticated route.
    ingress: {
      enabled: false,
    },
    preUpgradeChecker: {
      jobEnabled: false,
    },
    persistence: {
      defaultClass: true,
      defaultClassReplicaCount: 3,
      defaultDataLocality: "best-effort",
      defaultNodeSelector: {
        enable: true,
        selector: LONGHORN_STORAGE_NODE_TAG,
      },
    },
    defaultSettings: {
      createDefaultDiskLabeledNodes: true,
      defaultDataPath: "/var/lib/longhorn",
    },
  };
}
