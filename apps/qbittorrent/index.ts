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
import { Qbittorrent } from "./components/qbittorrent/index.js";
import { FLOOD_HTTP_PORT, QBITTORRENT_LABELS, QBIT_BRIDGE_PORT, QBIT_BRIDGE_SERVICE_NAME } from "./constants.js";

export * from "./lib/n8n-custom-nodes.js";

const QBITTORRENT_NAMESPACE = "qbittorrent";
const QBIT_BRIDGE_TOOL_NAMES = [
  "qbit_search_downloads",
  "qbit_add_download",
  "qbit_remove_downloads",
  "qbit_list_tags",
  "qbit_list_destinations",
  "qbit_search_subscriptions",
  "qbit_subscribe",
  "qbit_unsubscribe",
];

export const endpoints = {
  web(): BackendTarget {
    return { backend: qbittorrentEndpoint(), ports: [tcp(FLOOD_HTTP_PORT), tcp(QBIT_BRIDGE_PORT)] };
  },

  bridge(): PrivateConnectionTarget {
    return {
      to: qbittorrentEndpoint(),
      ports: [tcp(QBIT_BRIDGE_PORT)],
    };
  },

  mcp(): PrivateConnectionTarget {
    return endpoints.bridge();
  },
};

export const mcpServers = {
  qbitBridge(): McpServer {
    return {
      name: QBIT_BRIDGE_SERVICE_NAME,
      description: "qbit-bridge qBittorrent automation MCP server.",
      url: `http://${QBIT_BRIDGE_SERVICE_NAME}.${QBITTORRENT_NAMESPACE}.svc.cluster.local/mcp`,
      connection: endpoints.mcp(),
      toolNames: QBIT_BRIDGE_TOOL_NAMES,
    };
  },
};

function qbittorrentEndpoint(): PolicyEndpoint {
  return endpoint(QBITTORRENT_NAMESPACE, QBITTORRENT_LABELS, "qbittorrent");
}

export const createAppResources: AppResourceFunc = app => {
  new Qbittorrent(app, "qbittorrent");
  new NetworkPolicy(app, "network-policy");
};
