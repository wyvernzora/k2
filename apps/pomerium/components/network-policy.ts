import type { Construct } from "constructs";

import { ApexDomain, ClusterContext, K2Chart } from "@k2/cdk-lib";
import * as postgresql from "@k2/postgresql";
import { PrometheusPodScrape } from "@k2/prometheus";
import * as tailscale from "@k2/tailscale";

import {
  egress,
  EndpointNetworkPolicy,
  fqdn,
  ingress,
  NamespaceBoundaryPolicy,
  PrivateConnection,
  tcp,
} from "../../cilium/lib/netpol/index.js";
import { POMERIUM_IDP_HOST_PREFIX, POMERIUM_PROXY_HTTP_PORT, POMERIUM_PROXY_HTTPS_PORT } from "../constants.js";
import { workloads } from "../index.js";

import { MCP_CLIENT_METADATA_EGRESS_HOSTS } from "./global-config.js";

const POSTGRES_DNS_HOSTS = [
  fqdn.name("nexus-rw.postgresql.svc"),
  fqdn.name("nexus-rw.postgresql.svc.cluster.local"),
  fqdn.name("nexus-rw.postgresql.svc.pomerium.svc.cluster.local"),
  fqdn.name("nexus-rw.postgresql.svc.svc.cluster.local"),
];
const RFC1918_CIDRS = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"];

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const idpHost = ApexDomain.of(this).subdomain(POMERIUM_IDP_HOST_PREFIX);
    const authMetadataHosts = [idpHost, ...MCP_CLIENT_METADATA_EGRESS_HOSTS];
    const proxy = workloads.proxy();
    const clusterSubnets = ClusterContext.of(this).config.kubernetes.subnets;
    const proxyIngressPorts = [tcp(POMERIUM_PROXY_HTTP_PORT), tcp(POMERIUM_PROXY_HTTPS_PORT)];

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "proxy-load-balancer-ingress", {
      endpoint: proxy,
      ingress: [
        ...ingress.fromNodes(...proxyIngressPorts),
        ...ingress.fromKubeApiServer(...proxyIngressPorts),
        ...ingress.fromCluster(...proxyIngressPorts),
        ...ingress.fromCidrs([...RFC1918_CIDRS, clusterSubnets.pods, clusterSubnets.services], ...proxyIngressPorts),
        ...ingress.fromCidrTarget(tailscale.endpoints.tailnetClients(...proxyIngressPorts)),
      ],
    });
    new EndpointNetworkPolicy(this, "controller-kube-api-egress", {
      endpoint: proxy,
      egress: egress.toKubeApiServer(tcp(443), tcp(6443)),
    });
    new EndpointNetworkPolicy(this, "controller-oidc-egress", {
      endpoint: proxy,
      egress: [...egress.toDns(...authMetadataHosts), ...egress.toFqdns(authMetadataHosts, tcp(443))],
    });
    new EndpointNetworkPolicy(this, "controller-postgresql-dns-egress", {
      endpoint: proxy,
      egress: egress.toDns(...POSTGRES_DNS_HOSTS),
    });
    new PrivateConnection(this, "controller-to-postgresql", {
      from: proxy,
      ...postgresql.endpoints.nexus(),
    });
    new PrometheusPodScrape(this, "pomerium-metrics", {
      target: proxy,
      ports: [tcp(9090)],
    });
  }
}
