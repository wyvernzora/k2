import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import * as postgresql from "@k2/postgresql";

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

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const idpHost = ApexDomain.of(this).subdomain(POMERIUM_IDP_HOST_PREFIX);
    const authMetadataHosts = [idpHost, ...MCP_CLIENT_METADATA_EGRESS_HOSTS];
    const proxy = workloads.proxy();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "proxy-load-balancer-ingress", {
      endpoint: proxy,
      ingress: [
        ...ingress.fromNodes(tcp(POMERIUM_PROXY_HTTP_PORT), tcp(POMERIUM_PROXY_HTTPS_PORT)),
        ...ingress.fromKubeApiServer(tcp(POMERIUM_PROXY_HTTP_PORT), tcp(POMERIUM_PROXY_HTTPS_PORT)),
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
  }
}
