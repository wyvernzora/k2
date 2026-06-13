import type { Construct } from "constructs";

import { ApexDomain, K2Chart } from "@k2/cdk-lib";
import * as postgresql from "@k2/postgresql";

import { egress, EndpointNetworkPolicy, PrivateConnection, tcp } from "../../cilium/lib/netpol/index.js";
import { POMERIUM_IDP_HOST_PREFIX } from "../constants.js";
import { workloads } from "../index.js";

import { MCP_CLIENT_METADATA_EGRESS_HOSTS } from "./global-config.js";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const idpHost = ApexDomain.of(this).subdomain(POMERIUM_IDP_HOST_PREFIX);
    const authMetadataHosts = [idpHost, ...MCP_CLIENT_METADATA_EGRESS_HOSTS];
    const proxy = workloads.proxy();

    new EndpointNetworkPolicy(this, "controller-kube-api-egress", {
      endpoint: proxy,
      egress: egress.toKubeApiServer(tcp(443), tcp(6443)),
    });
    new EndpointNetworkPolicy(this, "controller-oidc-egress", {
      endpoint: proxy,
      egress: [...egress.toDns(...authMetadataHosts), ...egress.toFqdns(authMetadataHosts, tcp(443))],
    });
    new PrivateConnection(this, "controller-to-postgresql", {
      from: proxy,
      ...postgresql.endpoints.nexus(),
    });
  }
}
