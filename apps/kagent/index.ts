import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { AnimeManagerAgent, AnimeReleaseSearchAgent } from "./components/agents/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { KAgentSystem } from "./components/system/index.js";
import { ANIME_MANAGER_AGENT_NAME, ANIME_RELEASE_SEARCH_AGENT_NAME, KAGENT_UI_PORT } from "./constants.js";

export * as crd from "./lib/crd.js";
export * from "./lib/agent.js";
export * from "./constants.js";

const KAGENT_NAMESPACE = "kagent";
const KAGENT_UI_LABELS = {
  "app.kubernetes.io/name": "kagent",
  "app.kubernetes.io/instance": "kagent",
  "app.kubernetes.io/component": "ui",
};
const ANIME_KURA_AGENT_LABELS = {
  "app.kubernetes.io/name": ANIME_MANAGER_AGENT_NAME,
  "app.kubernetes.io/managed-by": "kagent",
};
const ANIME_RELEASE_SEARCH_AGENT_LABELS = {
  "app.kubernetes.io/name": ANIME_RELEASE_SEARCH_AGENT_NAME,
  "app.kubernetes.io/managed-by": "kagent",
};
const KAGENT_MANAGED_AGENT_LABELS = {
  "app.kubernetes.io/managed-by": "kagent",
};

export const endpoints = {
  http(): BackendTarget {
    return { backend: uiEndpoint(), ports: [tcp(KAGENT_UI_PORT)] };
  },
};

export const workloads = {
  agents(): PolicyEndpoint {
    return endpoint(KAGENT_NAMESPACE, KAGENT_MANAGED_AGENT_LABELS, "kagent-agents");
  },
  animeManagerAgent(): PolicyEndpoint {
    return endpoint(KAGENT_NAMESPACE, ANIME_KURA_AGENT_LABELS, ANIME_MANAGER_AGENT_NAME);
  },
  animeReleaseSearchAgent(): PolicyEndpoint {
    return endpoint(KAGENT_NAMESPACE, ANIME_RELEASE_SEARCH_AGENT_LABELS, ANIME_RELEASE_SEARCH_AGENT_NAME);
  },
  namespace(): PolicyEndpoint {
    return endpoint(KAGENT_NAMESPACE, {}, "kagent");
  },
};

function uiEndpoint(): PolicyEndpoint {
  return endpoint(KAGENT_NAMESPACE, KAGENT_UI_LABELS, "kagent-ui");
}

export const createAppResources: AppResourceFunc = app => {
  new KAgentSystem(app, "system");
  new AnimeManagerAgent(app, "anime-manager");
  new AnimeReleaseSearchAgent(app, "anime-release-search");
  new NetworkPolicy(app, "network-policy");
};
