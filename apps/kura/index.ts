import type { AppResourceFunc } from "@k2/cdk-lib";

import {
  endpoint,
  tcp,
  type BackendTarget,
  type PolicyEndpoint,
  type PrivateConnectionTarget,
} from "../cilium/lib/netpol/index.js";

import { Kura } from "./components/kura/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { KURA_HTTP_PORT, KURA_LABELS, KURA_MCP_PORT, KURA_WEBUI_HTTP_PORT, KURA_WEBUI_LABELS } from "./constants.js";

export * from "./lib/n8n-custom-nodes.js";

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

  webui(): BackendTarget {
    return { backend: webuiEndpoint(), ports: [tcp(KURA_WEBUI_HTTP_PORT)] };
  },
};

function kuraEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_LABELS, "kura");
}

function webuiEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_WEBUI_LABELS, "kura-webui");
}

export const createAppResources: AppResourceFunc = app => {
  new Kura(app, "kura");
  new NetworkPolicy(app, "network-policy");
};
