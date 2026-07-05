import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, cidr, egress, endpoint, ingress, tcp } from "@k2/cilium";

import { PLEX_ALLOW_VLANS, PLEX_CADDY_HTTP_REDIRECT_PORT, PLEX_CADDY_PORT, PLEX_HTTP_PORT } from "../constants.js";
import { workloads } from "../index.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const plex = workloads.plex();
    const plexClientCidrs = cidr.vlans(this, PLEX_ALLOW_VLANS);

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "plex-network", {
      endpoint: plex,
      ingress: [
        ...ingress.fromCidrs(plexClientCidrs, tcp(PLEX_CADDY_HTTP_REDIRECT_PORT)),
        ...ingress.fromCidrs(plexClientCidrs, tcp(PLEX_CADDY_PORT)),
        ...ingress.fromWorld(tcp(PLEX_CADDY_HTTP_REDIRECT_PORT)),
        ...ingress.fromWorld(tcp(PLEX_CADDY_PORT)),
        ...ingress.fromNodes(tcp(PLEX_CADDY_PORT), tcp(PLEX_HTTP_PORT)),
      ],
      ingressDeny: [
        {
          from: { endpoint: endpoint(namespace, {}, "plex-namespace") },
          ports: [tcp(PLEX_HTTP_PORT)],
        },
      ],
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toWorld(tcp(80), tcp(443))],
    });
  }
}
