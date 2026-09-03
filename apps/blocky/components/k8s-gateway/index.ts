import type { Construct } from "constructs";

import { ApexDomain, ClusterContext, HelmCharts, K2Chart, primaryKubernetesApiVip } from "@k2/cdk-lib";

import { k8sGatewayValues } from "./chart-values.js";
import { defaultCustomDns } from "./defaults.js";

export interface K8sGatewayProps {
  readonly publicDnsServers: string[];
}

export class K8sGateway extends K2Chart {
  public constructor(scope: Construct, id: string, props: K8sGatewayProps) {
    super(scope, id);

    const apexDomain = ApexDomain.of(this).apexDomain;
    const cluster = ClusterContext.of(this).config;
    const kubernetesApi = primaryKubernetesApiVip(cluster.kubernetes.api);
    const customDns = defaultCustomDns({
      apexDomain,
      kubernetesApi: kubernetesApi.address,
      staticRecords: cluster.dns.staticRecords,
    });

    HelmCharts.of(this).asChart(
      this,
      "k8s-gateway",
      "k8s-gateway",
      k8sGatewayValues({
        apexDomain,
        customDns,
        publicDnsServers: props.publicDnsServers,
        serviceClusterIp: cluster.dns.k8sGatewayServiceIp,
      }),
    );
  }
}
