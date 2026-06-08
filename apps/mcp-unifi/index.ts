import type { AppResourceFunc } from "@k2/cdk-lib";
import type { McpServer } from "@k2/kagent";

import {
  endpoint,
  tcp,
  type BackendTarget,
  type PolicyEndpoint,
  type PrivateConnectionTarget,
} from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { UnifiNetworkMcp } from "./components/unifi-network-mcp/index.js";
import { UNIFI_NETWORK_MCP_LABELS, UNIFI_NETWORK_MCP_PORT, UNIFI_NETWORK_MCP_SERVICE_NAME } from "./constants.js";

const MCP_UNIFI_NAMESPACE = "mcp-unifi";
const UNIFI_NETWORK_MCP_TOOL_NAMES = ["unifi_tool_index", "unifi_load_tools", "unifi_execute", "unifi_batch"];

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

export const mcpServers = {
  network(): McpServer {
    return {
      name: UNIFI_NETWORK_MCP_SERVICE_NAME,
      description: "UniFi Network management MCP server.",
      url: `http://${UNIFI_NETWORK_MCP_SERVICE_NAME}.${MCP_UNIFI_NAMESPACE}.svc.cluster.local/mcp`,
      connection: endpoints.mcp(),
      toolNames: UNIFI_NETWORK_MCP_TOOL_NAMES,
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
