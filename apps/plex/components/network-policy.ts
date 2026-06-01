import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, cidr, egress, endpoint, ingress, tcp } from "@k2/cilium";

import { PLEX_ALLOW_VLANS, PLEX_CADDY_PORT, PLEX_HTTP_PORT, PLEX_LABELS } from "./plex/labels.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const plex = endpoint(Namespace.of(this).namespace, PLEX_LABELS, "plex");
    const plexClientCidrs = cidr.vlans(this, PLEX_ALLOW_VLANS);

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "plex-network", {
      endpoint: plex,
      ingress: [
        ...ingress.fromCidrs(plexClientCidrs, tcp(PLEX_CADDY_PORT)),
        ...ingress.fromWorld(tcp(PLEX_CADDY_PORT)),
        ...ingress.fromHost(tcp(PLEX_CADDY_PORT), tcp(PLEX_HTTP_PORT)),
      ],
      ingressDeny: [
        {
          from: { endpoint: endpoint(Namespace.of(this).namespace, {}, "plex-namespace") },
          ports: [tcp(PLEX_HTTP_PORT)],
        },
      ],
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toWorld(tcp(80), tcp(443))],
    });
  }
}
