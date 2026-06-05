import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints } from "../index.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";
const TVDB_API_HOST = "api4.thetvdb.com";
const DMHY_HOST = "share.dmhy.org";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const kura = endpoints.httpAndMcp();
    const dmhyMcp = endpoints.dmhyMcpHttp();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-kura", {
      ...kura,
    });
    new AllowPomeriumToBackend(this, "pomerium-to-dmhy-mcp", {
      ...dmhyMcp,
    });
    new EndpointNetworkPolicy(this, "kura-egress", {
      endpoint: kura.backend,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toFqdns([TVDB_API_HOST], tcp(443))],
    });
    new EndpointNetworkPolicy(this, "dmhy-mcp-egress", {
      endpoint: dmhyMcp.backend,
      egress: [...egress.toFqdns([DMHY_HOST], tcp(443))],
    });
  }
}
