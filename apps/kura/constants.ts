export const KURA_HTTP_PORT = 8080;
export const KURA_MCP_PORT = 8081;
export const KURA_SERVICE_NAME = "kura";
export const KURA_MCP_SERVICE_NAME = "kura-mcp";

export const KURA_LABELS = {
  "app.kubernetes.io/name": "kura",
  "app.kubernetes.io/component": "library-manager",
};

export const DMHY_MCP_PORT = 8080;
export const DMHY_MCP_SERVICE_NAME = "dmhy-mcp";

export const DMHY_MCP_LABELS = {
  "app.kubernetes.io/name": "dmhy-mcp",
  "app.kubernetes.io/component": "search-acquisition",
};
