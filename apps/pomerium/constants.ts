export const POMERIUM_NAMESPACE = "pomerium";
export const POMERIUM_INGRESS_CLASS_NAME = "pomerium";
export const POMERIUM_CONTROLLER_NAME = "pomerium-controller";
export const POMERIUM_BOOTSTRAP_SECRET_NAME = "bootstrap";
export const POMERIUM_AUTHENTICATE_HOST_PREFIX = "login";
export const POMERIUM_IDP_HOST_PREFIX = "id";
export const POMERIUM_IDP_SECRET_NAME = "pocket-id";
export const POMERIUM_PROXY_SERVICE_NAME = "pomerium-proxy";
export const POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME = "default-certificate";

export const POMERIUM_LABELS = {
  "app.kubernetes.io/component": "proxy",
  "app.kubernetes.io/name": "pomerium",
};
