import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";
import { EndpointNetworkPolicy, NamespaceBoundaryPolicy, PrivateConnection, egress, fqdn, tcp } from "@k2/cilium";
import * as kura from "@k2/kura";
import * as postgresql from "@k2/postgresql";
import * as qbittorrent from "@k2/qbittorrent";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { endpoints, workloads } from "../index.js";

const MODEL_PROVIDER_FQDNS = [
  fqdn.name("api.openai.com"),
  fqdn.name("api.anthropic.com"),
  fqdn.pattern("bedrock.*.amazonaws.com"),
  fqdn.pattern("bedrock-runtime.*.amazonaws.com"),
  fqdn.pattern("bedrock-agent.*.amazonaws.com"),
  fqdn.pattern("bedrock-agent-runtime.*.amazonaws.com"),
];

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const kagent = workloads.namespace();

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new AllowPomeriumToBackend(this, "pomerium-to-kagent-ui", {
      ...endpoints.http(),
    });
    new EndpointNetworkPolicy(this, "agent-model-provider-egress", {
      endpoint: workloads.agents(),
      egress: egress.toFqdns(MODEL_PROVIDER_FQDNS, tcp(443)),
    });
    new PrivateConnection(this, "kagent-to-kura-mcp", {
      from: kagent,
      ...kura.mcpServers.kura().connection,
    });
    new PrivateConnection(this, "kagent-to-dmhy-mcp", {
      from: kagent,
      ...kura.mcpServers.dmhy().connection,
    });
    new PrivateConnection(this, "kagent-to-qbit-bridge", {
      from: kagent,
      ...qbittorrent.mcpServers.qbitBridge().connection,
    });
    new PrivateConnection(this, "kagent-to-postgresql", {
      from: kagent,
      ...postgresql.endpoints.nexus(),
    });
  }
}
