export const POMERIUM_NAMESPACE = "pomerium";
export const POMERIUM_INGRESS_CLASS_NAME = "pomerium";
export const POMERIUM_CONTROLLER_NAME = "pomerium-controller";
export const POMERIUM_BOOTSTRAP_SECRET_NAME = "bootstrap";
export const POMERIUM_AUTHENTICATE_HOST_PREFIX = "login";
export const POMERIUM_IDP_HOST_PREFIX = "id";
export const POMERIUM_IDP_SECRET_NAME = "pocket-id";
export const POMERIUM_PROXY_SERVICE_NAME = "pomerium-proxy";
export const POMERIUM_PROXY_LOAD_BALANCER_IP = "10.10.13.1";
export const POMERIUM_PROXY_CLUSTER_IP = "10.43.62.173";
export const POMERIUM_PROXY_HTTP_PORT = 8080;
export const POMERIUM_PROXY_HTTPS_PORT = 8443;
export const POMERIUM_DEFAULT_CERTIFICATE_SECRET_NAME = "default-certificate";
export const POMERIUM_DATABASE_CLAIM_NAME = "pomerium";
export const POMERIUM_DATABASE_NAME = "pomerium";
export const POMERIUM_DATABASE_ROLE_NAME = "pomerium";
export const POMERIUM_DATABASE_SECRET_NAME = `${POMERIUM_DATABASE_CLAIM_NAME}-credentials`;
export const POMERIUM_DATABASE_STORAGE_SECRET_NAME = `${POMERIUM_DATABASE_CLAIM_NAME}-storage`;

export const POMERIUM_LABELS = {
  "app.kubernetes.io/component": "proxy",
  "app.kubernetes.io/name": "pomerium",
};
