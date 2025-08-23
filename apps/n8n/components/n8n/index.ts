import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, HttpIngressPathType, Ingress, IngressBackend, Service } from "cdk8s-plus-28";

import { AuthenticatedIngress } from "@k2/auth";
import { VolumesOf } from "@k2/cdk-lib";

import { N8NDeployment, N8NDeploymentProps } from "./deployment.js";

export interface N8NProps {
  readonly url: string;
  readonly volumes: VolumesOf<N8NDeploymentProps>;
}

export class N8N extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: N8NProps) {
    super(scope, id, {});

    const { hostname, pathname } = new URL(props.url);
    this.deployment = new N8NDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      name: "n8n",
      ports: [{ port: 80, targetPort: 5678 }],
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
