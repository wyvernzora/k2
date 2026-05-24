import type { Construct } from "constructs";

import { ClusterContext, K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, endpoint, fqdn, ingress, tcp } from "@k2/cilium";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const cluster = ClusterContext.of(this).config;
    const namespace = Namespace.of(this).namespace;

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "webhook-admission-ingress", {
      endpoint: endpoint(
        namespace,
        {
          "app.kubernetes.io/component": "webhook",
          "app.kubernetes.io/instance": "cert-manager",
          "app.kubernetes.io/name": "webhook",
        },
        "cert-manager-webhook",
      ),
      ingress: ingress.fromNodes(tcp(10250)),
    });
    new EndpointNetworkPolicy(this, "controller-external-egress", {
      endpoint: endpoint(
        namespace,
        {
          "app.kubernetes.io/component": "controller",
          "app.kubernetes.io/instance": "cert-manager",
          "app.kubernetes.io/name": "cert-manager",
        },
        "cert-manager-controller",
      ),
      egress: [
        ...egress.toDns(),
        ...egress.toFqdns([
          fqdn.name("acme-v02.api.letsencrypt.org"),
          fqdn.name("route53.amazonaws.com"),
          fqdn.name(`sts.${awsRegion(cluster.aws?.region)}.amazonaws.com`),
        ]),
      ],
    });
  }
}

function awsRegion(region: string | undefined): string {
  if (region === undefined || region === "") {
    throw new Error("CertManager requires clusters/v3.yaml aws.region");
  }
  return region;
}
