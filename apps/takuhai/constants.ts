export const TAKUHAI_HTTP_PORT = 8080;
export const TAKUHAI_CRAWLER_PORT = 8080;

export const TAKUHAI_MCP_SERVICE_NAME = "takuhai-mcp";

export const TAKUHAI_LABELS = {
  "app.kubernetes.io/name": "takuhai",
  "app.kubernetes.io/component": "service",
};

export const TAKUHAI_CRAWLER_DMHY_LABELS = {
  "app.kubernetes.io/name": "takuhai",
  "app.kubernetes.io/component": "crawler-dmhy",
};

export const TAKUHAI_CRAWLER_LABELS = TAKUHAI_CRAWLER_DMHY_LABELS;

export const TAKUHAI_CRAWLER_NYAA_LABELS = {
  "app.kubernetes.io/name": "takuhai",
  "app.kubernetes.io/component": "crawler-nyaa",
};
