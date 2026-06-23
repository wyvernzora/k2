import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { ApexDomain, ClusterContext, K2Chart } from "@k2/cdk-lib";

const COREDNS_NAMESPACE = "kube-system";
const CONFIG_MAP_NAME = "coredns-custom";
const APEX_ZONE_FORWARD_KEY = "apex.server";

export class CoreDnsForward extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id, { namespace: COREDNS_NAMESPACE });

    const cluster = ClusterContext.of(this).config;
    const apexDomain = ApexDomain.of(this).apexDomain;

    new ConfigMap(this, "config", {
      metadata: { name: CONFIG_MAP_NAME },
      data: {
        [APEX_ZONE_FORWARD_KEY]: renderCorefileServer(apexDomain, cluster.dns.k8sGatewayServiceIp),
      },
    });
  }
}

function renderCorefileServer(apexDomain: string, gatewayIp: string): string {
  return `${apexDomain}:53 {
    errors
    forward . ${gatewayIp}
    cache 30
    reload
}
`;
}
