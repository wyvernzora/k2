import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, HttpIngressPathType, Ingress, IngressBackend, Service } from "cdk8s-plus-32";

import { AuthenticatedIngress } from "@k2/auth";
import { VolumesOf } from "@k2/cdk-lib";

import { KavitaDeployment, KavitaDeploymentProps } from "./deployment.js";

export interface KavitaProps {
  readonly url: string;
  readonly volumes: VolumesOf<KavitaDeploymentProps>;
}

export class Kavita extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: KavitaProps) {
    super(scope, id, {});

    const { hostname, pathname } = new URL(props.url);

    this.deployment = new KavitaDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      name: "kavita",
      ports: [
        {
          port: 80,
          targetPort: 5000,
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
