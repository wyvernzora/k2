import type { AppResourceFunc } from "@k2/cdk-lib";

import {
  endpoint,
  tcp,
  type BackendTarget,
  type PolicyEndpoint,
  type PrivateConnectionTarget,
} from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { UnifiNetworkMcp } from "./components/unifi-network-mcp/index.js";
import { UNIFI_NETWORK_MCP_LABELS, UNIFI_NETWORK_MCP_PORT } from "./constants.js";

const MCP_UNIFI_NAMESPACE = "mcp-unifi";

export const endpoints = {
  http(): BackendTarget {
    return { backend: unifiNetworkMcpEndpoint(), ports: [tcp(UNIFI_NETWORK_MCP_PORT)] };
  },

  mcp(): PrivateConnectionTarget {
    return {
      to: unifiNetworkMcpEndpoint(),
      ports: [tcp(UNIFI_NETWORK_MCP_PORT)],
    };
  },
};

function unifiNetworkMcpEndpoint(): PolicyEndpoint {
  return endpoint(MCP_UNIFI_NAMESPACE, UNIFI_NETWORK_MCP_LABELS, "unifi-network-mcp");
}

export const createAppResources: AppResourceFunc = app => {
  new UnifiNetworkMcp(app, "unifi-network-mcp");
  new NetworkPolicy(app, "network-policy");
};
