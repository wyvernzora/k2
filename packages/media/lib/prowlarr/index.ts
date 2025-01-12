import { ProwlarrDeployment, ProwlarrDeploymentProps } from "./deployment";
import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { AuthenticatedIngress } from "@k2/authelia";

export interface ProwlarrProps {
  readonly host: string;
  readonly volumes: ProwlarrDeploymentProps["volumes"];
}

export class Prowlarr extends Construct {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: ProwlarrProps) {
    super(scope, id);
    this.deployment = new ProwlarrDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      ports: [{ port: 80, targetPort: 9696 }],
    });
    this.ingress = new AuthenticatedIngress(this, "ingr", {
      rules: [
        {
          host: props.host,
          backend: IngressBackend.fromService(this.service),
        },
      ],
    });
  }
}
