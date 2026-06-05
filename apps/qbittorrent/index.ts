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
import {
  FLOOD_HTTP_PORT,
  QBITTORRENT_LABELS,
  QBITTORRENT_MCP_PORT,
  QBITTORRENT_MCP_SERVICE_NAME,
} from "./constants.js";

const QBITTORRENT_NAMESPACE = "qbittorrent";
const QBITTORRENT_MCP_TOOL_NAMES = ["qbit_add_download", "qbit_search_downloads", "qbit_remove_downloads"];

export const endpoints = {
  web(): BackendTarget {
    return { backend: qbittorrentEndpoint(), ports: [tcp(FLOOD_HTTP_PORT), tcp(QBITTORRENT_MCP_PORT)] };
  },

  mcp(): PrivateConnectionTarget {
    return {
      to: qbittorrentEndpoint(),
      ports: [tcp(QBITTORRENT_MCP_PORT)],
    };
  },
};

export const mcpServers = {
  qbittorrent(): McpServer {
    return {
      name: QBITTORRENT_MCP_SERVICE_NAME,
      description: "qBittorrent download management MCP server.",
      url: `http://${QBITTORRENT_MCP_SERVICE_NAME}.${QBITTORRENT_NAMESPACE}.svc.cluster.local/mcp`,
      connection: endpoints.mcp(),
      toolNames: QBITTORRENT_MCP_TOOL_NAMES,
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
