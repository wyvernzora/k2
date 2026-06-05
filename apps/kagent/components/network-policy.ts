import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, PrivateConnection, egress, tcp } from "@k2/cilium";
import * as kura from "@k2/kura";
import * as postgresql from "@k2/postgresql";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints, workloads } from "../index.js";

const OPENAI_API_HOST = "api.openai.com";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const kagent = workloads.namespace();
    const animeKuraAgent = workloads.animeKuraAgent();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-kagent-ui", {
      ...endpoints.http(),
    });
    new EndpointNetworkPolicy(this, "anime-kura-agent-openai-egress", {
      endpoint: animeKuraAgent,
      egress: egress.toFqdns([OPENAI_API_HOST], tcp(443)),
    });
    new PrivateConnection(this, "kagent-to-kura-mcp", {
      from: kagent,
      ...kura.endpoints.mcp(),
    });
    new PrivateConnection(this, "kagent-to-postgresql", {
      from: kagent,
      ...postgresql.endpoints.nexus(),
    });
  }
}
