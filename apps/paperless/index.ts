import type { AppResourceFunc, ArgoApplicationConfigFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { Paperless } from "./components/paperless/index.js";
import { PAPERLESS_HTTP_PORT, PAPERLESS_LABELS, PAPERLESS_MCP_PORT } from "./constants.js";

const PAPERLESS_NAMESPACE = "paperless";

export const endpoints = {
  http(): BackendTarget {
    return { backend: paperlessEndpoint(), ports: [tcp(PAPERLESS_HTTP_PORT)] };
  },

  mcp(): BackendTarget {
    return { backend: paperlessEndpoint(), ports: [tcp(PAPERLESS_MCP_PORT)] };
  },
};

function paperlessEndpoint(): PolicyEndpoint {
  return endpoint(PAPERLESS_NAMESPACE, PAPERLESS_LABELS, "paperless");
}

export const configureArgoApplication: ArgoApplicationConfigFunc = app => ({
  ignoreDifferences: [
    {
      kind: "Secret",
      name: "paperless-mcp-token",
      namespace: app.destinationNamespace,
      jsonPointers: ["/data"],
    },
  ],
});

export const createAppResources: AppResourceFunc = app => {
  new Paperless(app, "paperless");
  new NetworkPolicy(app, "network-policy");
};
