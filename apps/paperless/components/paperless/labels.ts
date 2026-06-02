export const PAPERLESS_HTTP_PORT = 8000;
export const REDIS_PORT = 6379;

export const PAPERLESS_SERVICE_NAME = "paperless";
export const REDIS_SERVICE_NAME = "paperless-redis";

export const PAPERLESS_LABELS = {
  "app.kubernetes.io/name": "paperless",
  "app.kubernetes.io/component": "app",
};

export const REDIS_LABELS = {
  "app.kubernetes.io/name": "paperless",
  "app.kubernetes.io/component": "redis",
};
