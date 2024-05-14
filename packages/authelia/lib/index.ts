import { Ingress, IngressProps } from "cdk8s-plus-28";
import { Construct } from "constructs";

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

/**
 * Ingress that uses Authelia middleware to require authentication.
 */
export class AuthenticatedIngress extends Ingress {
  constructor(scope: Construct, name: string, props: IngressProps) {
    super(scope, name, props);
    this.metadata.addAnnotation(
      "traefik.ingress.kubernetes.io/router.middlewares",
      MiddlewareName,
    );
  }
}
