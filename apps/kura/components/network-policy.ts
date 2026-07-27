import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, egress, tcp } from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { KURA_MCP_PORT } from "../constants.js";
import { endpoints } from "../index.js";

const TRUENAS_NFS_CIDR = "10.10.8.1/32";
const TVDB_API_HOST = "api4.thetvdb.com";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const kura = endpoints.httpAndMcp();
    const webui = endpoints.webui();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-kura-webui", {
      ...webui,
    });
    new AllowPomeriumToBackend(this, "pomerium-to-kura-mcp", {
      backend: kura.backend,
      ports: [tcp(KURA_MCP_PORT)],
    });
    new EndpointNetworkPolicy(this, "kura-egress", {
      endpoint: kura.backend,
      egress: [...egress.toCidrs([TRUENAS_NFS_CIDR], tcp(2049)), ...egress.toFqdns([TVDB_API_HOST], tcp(443))],
    });
  }
}
