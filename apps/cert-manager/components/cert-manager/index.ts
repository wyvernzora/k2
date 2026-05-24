import { ServiceAccount } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { HelmCharts, K2Chart } from "@k2/cdk-lib";

import { DefaultWildcardCertificate } from "./certificate.js";
import { ROUTE53_DNS01_SERVICE_ACCOUNT_NAME } from "./constants.js";
import { LetsEncryptClusterIssuer } from "./issuer.js";
import { Route53TokenRequestRbac } from "./route53-token-request-rbac.js";

export class CertManager extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const route53ServiceAccount = new ServiceAccount(this, "route53-service-account", {
      metadata: { name: ROUTE53_DNS01_SERVICE_ACCOUNT_NAME },
      automountToken: false,
    });
    new Route53TokenRequestRbac(this, "route53-token-request-rbac", {
      serviceAccountName: route53ServiceAccount.name,
    });

    HelmCharts.of(this).asChart(this, "cert-manager", "cert-manager", {
      crds: {
        enabled: false,
      },
    });

    const issuer = new LetsEncryptClusterIssuer(this, "issuer", {
      route53ServiceAccountName: route53ServiceAccount.name,
    });
    new DefaultWildcardCertificate(this, "default-certificate", { issuerName: issuer.name });
  }
}
