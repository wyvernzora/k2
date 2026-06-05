import type { AppResourceFunc } from "@k2/cdk-lib";
import type { McpServer } from "@k2/kagent";

import {
  endpoint,
  tcp,
  type BackendTarget,
  type PolicyEndpoint,
  type PrivateConnectionTarget,
} from "../cilium/lib/netpol/index.js";

import { DmhyMcp } from "./components/dmhy-mcp/index.js";
import { Kura } from "./components/kura/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { DMHY_MCP_LABELS, DMHY_MCP_PORT, KURA_HTTP_PORT, KURA_LABELS, KURA_MCP_PORT } from "./constants.js";
import { DMHY_MCP_SERVICE_NAME, KURA_MCP_SERVICE_NAME } from "./constants.js";

const KURA_NAMESPACE = "kura";
const KURA_MCP_TOOL_NAMES = [
  "kura_resolve",
  "kura_aliases",
  "kura_list",
  "kura_show",
  "kura_inbox_list",
  "kura_job_status",
  "kura_reconcile_plan",
  "kura_add",
  "kura_import",
  "kura_scan",
  "kura_stage",
  "kura_reset",
  "kura_reconcile_apply",
];
const DMHY_MCP_TOOL_NAMES = ["search_releases", "get_magnets"];

export const endpoints = {
  http(): BackendTarget {
    return { backend: kuraEndpoint(), ports: [tcp(KURA_HTTP_PORT)] };
  },

  httpAndMcp(): BackendTarget {
    return { backend: kuraEndpoint(), ports: [tcp(KURA_HTTP_PORT), tcp(KURA_MCP_PORT)] };
  },

  mcp(): PrivateConnectionTarget {
    return {
      to: kuraEndpoint(),
      ports: [tcp(KURA_MCP_PORT)],
    };
  },

  dmhyMcpHttp(): BackendTarget {
    return { backend: dmhyMcpEndpoint(), ports: [tcp(DMHY_MCP_PORT)] };
  },

  dmhyMcp(): PrivateConnectionTarget {
    return {
      to: dmhyMcpEndpoint(),
      ports: [tcp(DMHY_MCP_PORT)],
    };
  },
};

export const mcpServers = {
  kura(): McpServer {
    return {
      name: KURA_MCP_SERVICE_NAME,
      description: "Kura anime library MCP server.",
      url: mcpUrl(KURA_MCP_SERVICE_NAME),
      connection: endpoints.mcp(),
      toolNames: KURA_MCP_TOOL_NAMES,
    };
  },

  dmhy(): McpServer {
    return {
      name: DMHY_MCP_SERVICE_NAME,
      description: "DMHY anime release search MCP server.",
      url: mcpUrl(DMHY_MCP_SERVICE_NAME),
      connection: endpoints.dmhyMcp(),
      toolNames: DMHY_MCP_TOOL_NAMES,
    };
  },
};

function kuraEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_LABELS, "kura");
}

function dmhyMcpEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, DMHY_MCP_LABELS, "dmhy-mcp");
}

function mcpUrl(serviceName: string): string {
  return `http://${serviceName}.${KURA_NAMESPACE}.svc.cluster.local/mcp`;
}

export const createAppResources: AppResourceFunc = app => {
  new Kura(app, "kura");
  new DmhyMcp(app, "dmhy-mcp");
  new NetworkPolicy(app, "network-policy");
};
