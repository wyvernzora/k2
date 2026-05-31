import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, endpoint, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { DMHY_MCP_LABELS, DMHY_MCP_PORT } from "./dmhy-mcp/labels.js";
import { KURA_HTTP_PORT, KURA_LABELS, KURA_MCP_PORT } from "./kura/labels.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";
const TVDB_API_HOST = "api4.thetvdb.com";
const DMHY_HOST = "share.dmhy.org";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const kura = endpoint(Namespace.of(this).namespace, KURA_LABELS, "kura");
    const dmhyMcp = endpoint(Namespace.of(this).namespace, DMHY_MCP_LABELS, "dmhy-mcp");

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-kura", {
      backend: kura,
      ports: [tcp(KURA_HTTP_PORT), tcp(KURA_MCP_PORT)],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-dmhy-mcp", {
      backend: dmhyMcp,
      ports: [tcp(DMHY_MCP_PORT)],
    });
    new EndpointNetworkPolicy(this, "kura-egress", {
      endpoint: kura,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toFqdns([TVDB_API_HOST], tcp(443))],
    });
    new EndpointNetworkPolicy(this, "dmhy-mcp-egress", {
      endpoint: dmhyMcp,
      egress: [...egress.toFqdns([DMHY_HOST], tcp(443))],
    });
  }
}
