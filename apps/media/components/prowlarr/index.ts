import { Deployment, HttpIngressPathType, Ingress, IngressBackend, Service } from "cdk8s-plus-28";
import { Construct } from "constructs";

import { AuthenticatedIngress } from "@k2/auth";

import { ProwlarrDeployment, ProwlarrDeploymentProps } from "./deployment.js";

export interface ProwlarrProps {
  readonly url: string;
  readonly volumes: ProwlarrDeploymentProps["volumes"];
}

export class Prowlarr extends Construct {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: ProwlarrProps) {
    super(scope, id);

    const { hostname, pathname } = new URL(props.url);

    this.deployment = new ProwlarrDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      name: "prowlarr", // Need to explicitly name this for integration with other apps
      ports: [{ port: 80, targetPort: 9696 }],
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
