export const PLEX_HTTP_PORT = 32400;
export const PLEX_CADDY_PORT = 8443;
export const PLEX_ALLOW_VLANS = ["default", "privileged", "infrastructure"];

export const PLEX_LABELS = {
  "app.kubernetes.io/name": "plex",
  "app.kubernetes.io/component": "media-server",
};
