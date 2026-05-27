import type { Construct } from "constructs";

import { HelmCharts, K2Chart, TopologySpread } from "@k2/cdk-lib";

export class CNPG extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    HelmCharts.of(this).asChart(this, "cnpg", "cloudnative-pg", cnpgValues());
  }
}

function cnpgValues() {
  return {
    replicaCount: 2,

    // CRDs ship via apps/postgresql/crds/crds.k8s.yaml.
    crds: { create: false },

    config: { clusterWide: false },
    rbac: { create: true },
    serviceAccount: { create: true },

    resources: {
      requests: { cpu: "50m", memory: "128Mi" },
      limits: { cpu: "500m", memory: "512Mi" },
    },

    topologySpreadConstraints: [
      TopologySpread.acrossZones({
        matchLabels: {
          "app.kubernetes.io/name": "cloudnative-pg",
        },
      }),
      TopologySpread.acrossHosts({
        matchLabels: {
          "app.kubernetes.io/name": "cloudnative-pg",
        },
      }),
    ],

    monitoring: {
      podMonitorEnabled: false,
      grafanaDashboard: { create: false },
    },
  };
}
