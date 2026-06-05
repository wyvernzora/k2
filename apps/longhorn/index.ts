import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { Longhorn } from "./components/longhorn.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * as crd from "./lib/crd.js";

const LONGHORN_NAMESPACE = "longhorn";
const LONGHORN_UI_HTTP_PORT = 8000;
const LONGHORN_UI_LABELS = {
  app: "longhorn-ui",
};

export const endpoints = {
  http(): BackendTarget {
    return { backend: longhornUiEndpoint(), ports: [tcp(LONGHORN_UI_HTTP_PORT)] };
  },
};

function longhornUiEndpoint(): PolicyEndpoint {
  return endpoint(LONGHORN_NAMESPACE, LONGHORN_UI_LABELS, "longhorn-ui");
}

export const createAppResources: AppResourceFunc = app => {
  new Longhorn(app, "longhorn");
  new NetworkPolicy(app, "network-policy");
};
