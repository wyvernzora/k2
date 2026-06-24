import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { NetworkPolicy } from "./components/network-policy.js";
import { Takuhai } from "./components/takuhai/index.js";
import { TAKUHAI_CRAWLER_LABELS, TAKUHAI_CRAWLER_PORT, TAKUHAI_HTTP_PORT, TAKUHAI_LABELS } from "./constants.js";

export * from "./constants.js";
export * from "./lib/n8n-custom-nodes.js";

export const endpoints = {
  http(): BackendTarget {
    return { backend: takuhaiEndpoint(), ports: [tcp(TAKUHAI_HTTP_PORT)] };
  },

  crawler(): BackendTarget {
    return { backend: crawlerEndpoint(), ports: [tcp(TAKUHAI_CRAWLER_PORT)] };
  },
};

export const workloads = {
  takuhai(): PolicyEndpoint {
    return takuhaiEndpoint();
  },

  crawler(): PolicyEndpoint {
    return crawlerEndpoint();
  },
};

function takuhaiEndpoint(): PolicyEndpoint {
  return endpoint("takuhai", TAKUHAI_LABELS, "takuhai");
}

function crawlerEndpoint(): PolicyEndpoint {
  return endpoint("takuhai", TAKUHAI_CRAWLER_LABELS, "crawler-dmhy");
}

export const createAppResources: AppResourceFunc = app => {
  new Takuhai(app, "takuhai");
  new NetworkPolicy(app, "network-policy");
};
