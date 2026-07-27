export const KURA_HTTP_PORT = 8080;
export const KURA_MCP_PORT = 8081;
export const KURA_SERVICE_NAME = "kura";
export const KURA_MCP_SERVICE_NAME = "kura-mcp";
export const KURA_RELEASE_INDEXER_HTTP_PORT = 8080;
export const KURA_WEBUI_HTTP_PORT = 8080;
export const KURA_WEBUI_SERVICE_NAME = "kura-webui";

export const KURA_LABELS = {
  "app.kubernetes.io/name": "kura",
  "app.kubernetes.io/component": "library-manager",
};

export const KURA_WEBUI_LABELS = {
  "app.kubernetes.io/name": "kura",
  "app.kubernetes.io/component": "webui",
};

export const KURA_RELEASE_INDEXER_LABELS = {
  "app.kubernetes.io/name": "kura",
  "app.kubernetes.io/component": "release-indexer",
};
