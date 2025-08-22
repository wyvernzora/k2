import { HelmChartV1 } from "@k2/cdk-lib";
import { Construct } from "constructs";
import { K2Issuer, K2Certificate } from "@k2/cert-manager";
import * as OnePassword from "@k2/1password";

export interface CertManagerProps {
  readonly namespace: string;
  readonly awsSecretId: string;
}

export class CertManager extends HelmChartV1 {
  readonly issuer: K2Issuer;
  readonly cert: K2Certificate;

  constructor(scope: Construct, name: string, props: CertManagerProps) {
    super(scope, name, {
      namespace: props.namespace,
      chart: "helm:https://charts.jetstack.io/cert-manager@v1.18.2",
      values: {
        installCRDs: true,
        extraArgs: ["--controllers=*"],
      },
    });

    this.issuer = new K2Issuer(this, "issuer", {
      credentials: new OnePassword.K2Secret(this, "aws-credentials", {
        itemId: "hxitqr6xcco7g2ne3n7m6kkoqa",
      }),
    });

    this.cert = new K2Certificate(this, "cert", { issuer: this.issuer });
  }
}
