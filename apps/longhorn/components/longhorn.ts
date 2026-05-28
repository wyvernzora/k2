import type { Construct } from "constructs";

import { HelmCharts, K2Chart } from "@k2/cdk-lib";

const LONGHORN_STORAGE_NODE_TAG = "k2-storage";

export class Longhorn extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    HelmCharts.of(this).asChart(this, "longhorn", "longhorn", longhornValues());
  }
}

function longhornValues() {
  return {
    // TODO: expose the Longhorn dashboard through Pomerium once the v3 auth
    // ingress surface is ready.
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
