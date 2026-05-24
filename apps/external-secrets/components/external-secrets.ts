import type { Construct } from "constructs";

import { HelmCharts, K2Chart } from "@k2/cdk-lib";

/**
 * External Secrets Operator Helm chart.
 */
export class ExternalSecrets extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    HelmCharts.of(this).asChart(this, "external-secrets", "external-secrets", {
      // CRDs ship via apps/external-secrets/crds/crds.k8s.yaml — disable
      // Helm-side rendering to avoid duplicate manifests at apply time.
      installCRDs: false,
    });
  }
}
