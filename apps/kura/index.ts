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
import { ReleaseIndexer } from "./components/release-indexer/index.js";
import {
  KURA_HTTP_PORT,
  KURA_LABELS,
  KURA_MCP_PORT,
  KURA_RELEASE_INDEXER_HTTP_PORT,
  KURA_RELEASE_INDEXER_LABELS,
  KURA_WEBUI_HTTP_PORT,
  KURA_WEBUI_LABELS,
} from "./constants.js";

export * from "./lib/n8n-custom-nodes.js";

const KURA_NAMESPACE = "kura";

export const endpoints = {
  http(): BackendTarget {
    return { backend: libraryManagerEndpoint(), ports: [tcp(KURA_HTTP_PORT)] };
  },

  httpAndMcp(): BackendTarget {
    return { backend: libraryManagerEndpoint(), ports: [tcp(KURA_HTTP_PORT), tcp(KURA_MCP_PORT)] };
  },

  mcp(): PrivateConnectionTarget {
    return {
      to: libraryManagerEndpoint(),
      ports: [tcp(KURA_MCP_PORT)],
    };
  },

  releaseIndexer(): BackendTarget {
    return {
      backend: releaseIndexerEndpoint(),
      ports: [tcp(KURA_RELEASE_INDEXER_HTTP_PORT)],
    };
  },

  releaseIndexerMcp(): PrivateConnectionTarget {
    return {
      to: releaseIndexerEndpoint(),
      ports: [tcp(KURA_RELEASE_INDEXER_HTTP_PORT)],
    };
  },

  webui(): BackendTarget {
    return { backend: webuiEndpoint(), ports: [tcp(KURA_WEBUI_HTTP_PORT)] };
  },
};

export const workloads = {
  libraryManager(): PolicyEndpoint {
    return libraryManagerEndpoint();
  },

  releaseIndexer(): PolicyEndpoint {
    return releaseIndexerEndpoint();
  },

  webui(): PolicyEndpoint {
    return webuiEndpoint();
  },
};

function libraryManagerEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_LABELS, "kura-library-manager");
}

function releaseIndexerEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_RELEASE_INDEXER_LABELS, "kura-release-indexer");
}

function webuiEndpoint(): PolicyEndpoint {
  return endpoint(KURA_NAMESPACE, KURA_WEBUI_LABELS, "kura-webui");
}

export const createAppResources: AppResourceFunc = app => {
  new Kura(app, "kura");
  new ReleaseIndexer(app, "release-indexer");
  new NetworkPolicy(app, "network-policy");
};
