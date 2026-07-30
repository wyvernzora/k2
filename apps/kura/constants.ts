export const KURA_GATEWAY_HTTP_PORT = 8080;
export const KURA_GATEWAY_METRICS_PORT = 9090;
export const KURA_GATEWAY_MCP_METRICS_PORT = 9091;
export const KURA_SERVICE_NAME = "kura";
export const KURA_LIBRARY_MANAGER_HTTP_PORT = 8080;
export const KURA_LIBRARY_MANAGER_METRICS_PORT = 9090;
export const KURA_LIBRARY_MANAGER_SERVICE_NAME = "kura-library-manager";
export const KURA_RELEASE_INDEXER_HTTP_PORT = 8080;
export const KURA_RELEASE_INDEXER_METRICS_PORT = 9090;
export const KURA_RELEASE_INDEXER_SERVICE_NAME = "kura-release-indexer";

export const KURA_GATEWAY_LABELS = {
  "app.kubernetes.io/name": "kura",
  "app.kubernetes.io/component": "gateway",
};

export const KURA_LIBRARY_MANAGER_LABELS = {
  "app.kubernetes.io/name": "kura",
  "app.kubernetes.io/component": "library-manager",
};

export const KURA_RELEASE_INDEXER_LABELS = {
  "app.kubernetes.io/name": "kura",
  "app.kubernetes.io/component": "release-indexer",
};
