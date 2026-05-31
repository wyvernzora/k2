import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, endpoint, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { FLOOD_HTTP_PORT, QBITTORRENT_LABELS, QBITTORRENT_MCP_PORT } from "./qbittorrent/labels.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const qbittorrent = endpoint(Namespace.of(this).namespace, QBITTORRENT_LABELS, "qbittorrent");

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-qbittorrent", {
      backend: qbittorrent,
      ports: [tcp(FLOOD_HTTP_PORT), tcp(QBITTORRENT_MCP_PORT)],
    });
    new EndpointNetworkPolicy(this, "qbittorrent-egress", {
      endpoint: qbittorrent,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toWorld()],
    });
  }
}
