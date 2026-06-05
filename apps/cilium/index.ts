import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "./lib/netpol/index.js";
import { Cilium } from "./components/cilium/index.js";
import { NetworkPolicy } from "./components/network-policy.js";

export * from "./lib/netpol/index.js";
export * from "./lib/load-balancer-service.js";
export * from "./lib/vlans.js";

const CILIUM_NAMESPACE = "cilium";
const HUBBLE_UI_HTTP_PORT = 8081;
const HUBBLE_UI_LABELS = {
  "k8s-app": "hubble-ui",
};

export const endpoints = {
  hubbleUiHttp(): BackendTarget {
    return { backend: hubbleUiEndpoint(), ports: [tcp(HUBBLE_UI_HTTP_PORT)] };
  },
};

function hubbleUiEndpoint(): PolicyEndpoint {
  return endpoint(CILIUM_NAMESPACE, HUBBLE_UI_LABELS, "hubble-ui");
}

export const createAppResources: AppResourceFunc = app => {
  new Cilium(app, "cilium");
  new NetworkPolicy(app, "network-policy");
};
