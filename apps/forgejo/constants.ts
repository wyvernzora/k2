export const FORGEJO_NAMESPACE = "forgejo";
export const FORGEJO_SERVICE_NAME = "forgejo";
export const FORGEJO_HOST = "git.wyvernzora.io";
export const FORGEJO_HTTP_PORT = 3000;
export const FORGEJO_HTTP_REDIRECT_PORT = 80;
export const FORGEJO_HTTPS_PORT = 8443;
export const FORGEJO_SSH_PORT = 2222;
export const FORGEJO_ALLOW_VLANS = ["default", "privileged", "infrastructure"];
export const FORGEJO_OIDC_SECRET_NAME = "forgejo-oidc";
export const FORGEJO_OIDC_CLIENT_ID = "forgejo";

export const FORGEJO_LABELS = {
  "app.kubernetes.io/name": "forgejo",
  "app.kubernetes.io/component": "forge",
};
