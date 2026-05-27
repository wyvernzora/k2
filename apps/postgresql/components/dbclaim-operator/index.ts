import type { Construct } from "constructs";

import { HelmCharts, K2Chart } from "@k2/cdk-lib";

export class DbClaimOperator extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    HelmCharts.of(this).asChart(this, "dbclaim-operator", "dbclaim-operator", dbClaimOperatorValues());
  }
}

function dbClaimOperatorValues() {
  return {
    // CRDs ship via apps/postgresql/crds/crds.k8s.yaml.
    installCRDs: false,
    replicaCount: 2,
    leaderElection: true,

    resources: {
      requests: { cpu: "50m", memory: "64Mi" },
      limits: { cpu: "500m", memory: "256Mi" },
    },

    metrics: { enabled: false },
  };
}
