import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, HttpIngressPathType, Ingress, IngressBackend, Service } from "cdk8s-plus-32";

import { AuthenticatedIngress } from "@k2/auth";

import { KuraDeployment, KuraDeploymentProps } from "./deployment.js";

export interface KuraProps {
  readonly url: string;
  readonly volumes: KuraDeploymentProps["volumes"];
}

export class Kura extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly mcpService: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: KuraProps) {
    super(scope, id, {});

    const { hostname, pathname } = new URL(props.url);

    this.deployment = new KuraDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      name: "kura",
      ports: [
        {
          port: 80,
          targetPort: 8080,
        },
      ],
    });
    this.mcpService = this.deployment.exposeViaService({
      name: "kura-mcp",
      ports: [
        {
          port: 8081,
          targetPort: 8081,
        },
      ],
    });
    this.ingress = new AuthenticatedIngress(this, "ingress", {
      rules: [
        {
          host: hostname,
          path: pathname,
          pathType: HttpIngressPathType.PREFIX,
          backend: IngressBackend.fromService(this.service),
        },
      ],
    });
  }
}
