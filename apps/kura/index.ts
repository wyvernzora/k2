import type { AppResourceFunc } from "@k2/cdk-lib";

import { endpoint, tcp, type BackendTarget, type PolicyEndpoint } from "../cilium/lib/netpol/index.js";

import { Kura } from "./components/kura/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { ReleaseIndexer } from "./components/release-indexer/index.js";
import {
  KURA_GATEWAY_HTTP_PORT,
  KURA_GATEWAY_LABELS,
  KURA_LIBRARY_MANAGER_HTTP_PORT,
  KURA_LIBRARY_MANAGER_LABELS,
  KURA_RELEASE_INDEXER_HTTP_PORT,
  KURA_RELEASE_INDEXER_LABELS,
} from "./constants.js";

export * from "./lib/n8n-custom-nodes.js";

const KURA_NAMESPACE = "kura";

export const endpoints = {
  http(): BackendTarget {
    return { backend: gatewayEndpoint(), ports: [tcp(KURA_GATEWAY_HTTP_PORT)] };
  },

  libraryManager(): BackendTarget {
    return { backend: libraryManagerEndpoint(), ports: [tcp(KURA_LIBRARY_MANAGER_HTTP_PORT)] };
  },

  releaseIndexer(): BackendTarget {
    return {
      backend: releaseIndexerEndpoint(),
      ports: [tcp(KURA_RELEASE_INDEXER_HTTP_PORT)],
    };
  },
};

export const workloads = {
  gateway(): PolicyEndpoint {
    return gatewayEndpoint();
  },

  libraryManager(): PolicyEndpoint {
    return libraryManagerEndpoint();
  },

  releaseIndexer(): PolicyEndpoint {
    return releaseIndexerEndpoint();
  },
};

function gatewayEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_GATEWAY_LABELS, "kura-gateway");
}

function libraryManagerEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_LIBRARY_MANAGER_LABELS, "kura-library-manager");
}

function releaseIndexerEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_RELEASE_INDEXER_LABELS, "kura-release-indexer");
}

export const createAppResources: AppResourceFunc = app => {
  new Kura(app, "kura");
  new ReleaseIndexer(app, "release-indexer");
  new NetworkPolicy(app, "network-policy");
};
