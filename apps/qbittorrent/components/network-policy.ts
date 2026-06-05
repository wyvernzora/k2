import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const qbittorrentWeb = endpoints.web();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-qbittorrent", {
      ...qbittorrentWeb,
    });
    new EndpointNetworkPolicy(this, "qbittorrent-egress", {
      endpoint: qbittorrentWeb.backend,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toWorld()],
    });
  }
}
