import type { AppResourceFunc } from "@k2/cdk-lib";

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

const KURA_NAMESPACE = "kura";

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

  dmhyMcp(): BackendTarget {
    return { backend: dmhyMcpEndpoint(), ports: [tcp(DMHY_MCP_PORT)] };
  },
};

function kuraEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_LABELS, "kura");
}

function dmhyMcpEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, DMHY_MCP_LABELS, "dmhy-mcp");
}

export const createAppResources: AppResourceFunc = app => {
  new Kura(app, "kura");
  new DmhyMcp(app, "dmhy-mcp");
  new NetworkPolicy(app, "network-policy");
};
