/**
 * Name of the Traefik middleware that handles Authelia integration.
 */
export const MiddlewareName = "k2-auth-authelia@kubernetescrd";

/**
 * Partial annotation object that adds Authelia middleware to the ingress.
 */
export const MiddlewareAnnotation = {
  "traefik.ingress.kubernetes.io/router.middlewares": MiddlewareName,
};
