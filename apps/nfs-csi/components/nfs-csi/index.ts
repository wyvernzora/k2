import type { Construct } from "constructs";

import { HelmCharts, K2Chart, NfsContext } from "@k2/cdk-lib";

import { nfsCsiValues } from "./chart-values.js";
import { tightenRbac } from "./rbac.js";

export class NfsCsi extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const chart = HelmCharts.of(this).asChart(
      this,
      "csi-driver-nfs",
      "csi-driver-nfs",
      nfsCsiValues(NfsContext.of(this).server),
    );
    tightenRbac(this, chart);
  }
}
