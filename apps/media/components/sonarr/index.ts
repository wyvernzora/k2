import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, HttpIngressPathType, Ingress, IngressBackend, Service } from "cdk8s-plus-28";

import { AuthenticatedIngress } from "@k2/auth";

import { SonarrDeployment, SonarrDeploymentProps } from "./deployment";

export interface SonarrProps {
  readonly url: string;
  readonly volumes: SonarrDeploymentProps["volumes"];
}

export class Sonarr extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: SonarrProps) {
    super(scope, id, {});

    const { hostname, pathname } = new URL(props.url);

    this.deployment = new SonarrDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      name: "sonarr", // Need to explicitly name this for integration with other apps
      ports: [
        {
          port: 80,
          targetPort: 8989,
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
