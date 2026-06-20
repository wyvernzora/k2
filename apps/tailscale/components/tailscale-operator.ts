import type { Construct } from "constructs";

import { HelmCharts, K2Chart } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";

const OPERATOR_OAUTH_SECRET_NAME = "operator-oauth";
const OPERATOR_OAUTH_SECRET_ID = "4of6ip5wf5s4s5lj2z4bwxww7a";

export class TailscaleOperator extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new ManagedSecret(this, "operator-oauth", {
      metadata: { name: OPERATOR_OAUTH_SECRET_NAME },
      secretId: OPERATOR_OAUTH_SECRET_ID,
      fields: {
        client_id: "client_id",
        client_secret: "client_secret",
      },
    });

    HelmCharts.of(this).asChart(this, "tailscale-operator", "tailscale-operator", {
      installCRDs: false,
      oauth: {
        clientId: "",
        clientSecret: "",
      },
      apiServerProxyConfig: {
        mode: "false",
      },
    });
  }
}
