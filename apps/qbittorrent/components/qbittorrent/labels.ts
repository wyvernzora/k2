export const QBITTORRENT_HTTP_PORT = 8080;
export const FLOOD_HTTP_PORT = 3000;
export const QBITTORRENT_MCP_PORT = 8082;

export const FLOOD_SERVICE_NAME = "flood";
export const QBITTORRENT_MCP_SERVICE_NAME = "qbittorrent-mcp";
export const QBITTORRENT_SERVICE_NAME = "qbittorrent";

export const QBITTORRENT_LABELS = {
  "app.kubernetes.io/name": "qbittorrent",
  "app.kubernetes.io/component": "download-client",
};
