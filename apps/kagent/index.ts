import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { AnimeKuraAgent } from "./components/agent/index.js";
import { Kagent } from "./components/kagent/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { KAGENT_UI_PORT } from "./constants.js";

export * as crd from "./lib/crd.js";
export * from "./constants.js";

const KAGENT_NAMESPACE = "kagent";
const KAGENT_UI_LABELS = {
  "app.kubernetes.io/name": "kagent",
  "app.kubernetes.io/instance": "kagent",
  "app.kubernetes.io/component": "ui",
};
const ANIME_KURA_AGENT_LABELS = {
  "app.kubernetes.io/name": "anime-kura-agent",
  "app.kubernetes.io/managed-by": "kagent",
};

export const endpoints = {
  http(): BackendTarget {
    return { backend: uiEndpoint(), ports: [tcp(KAGENT_UI_PORT)] };
  },
};

export const workloads = {
  animeKuraAgent(): PolicyEndpoint {
    return endpoint(KAGENT_NAMESPACE, ANIME_KURA_AGENT_LABELS, "anime-kura-agent");
  },
  namespace(): PolicyEndpoint {
    return endpoint(KAGENT_NAMESPACE, {}, "kagent");
  },
};

function uiEndpoint(): PolicyEndpoint {
  return endpoint(KAGENT_NAMESPACE, KAGENT_UI_LABELS, "kagent-ui");
}

export const createAppResources: AppResourceFunc = app => {
  new Kagent(app, "kagent");
  new AnimeKuraAgent(app, "anime-kura-agent");
  new NetworkPolicy(app, "network-policy");
};
