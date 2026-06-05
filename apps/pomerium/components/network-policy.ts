import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";

import { egress, EndpointNetworkPolicy, tcp } from "../../cilium/lib/netpol/index.js";
import { POMERIUM_IDP_HOST_PREFIX } from "../constants.js";
import { workloads } from "../index.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const idpHost = ApexDomain.of(this).subdomain(POMERIUM_IDP_HOST_PREFIX);
    const proxy = workloads.proxy();

    new EndpointNetworkPolicy(this, "controller-kube-api-egress", {
      endpoint: proxy,
      egress: egress.toKubeApiServer(tcp(443), tcp(6443)),
    });
    new EndpointNetworkPolicy(this, "controller-oidc-egress", {
      endpoint: proxy,
      egress: [...egress.toDns(idpHost), ...egress.toFqdns([idpHost], tcp(443))],
    });
  }
}
