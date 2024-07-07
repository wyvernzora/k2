import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-28";
import { QBitTorrentDeployment, QBitTorrentDeploymentProps } from "./deployment";
import { Construct } from "constructs";
import { AuthenticatedIngress } from "@k2/authelia";

export interface QBitTorrentProps {
  readonly host: string;
  readonly volumes: QBitTorrentDeploymentProps["volumes"];
  readonly downloads: QBitTorrentDeploymentProps["downloads"];
}

export class QBitTorrent extends Construct {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: QBitTorrentProps) {
    super(scope, id);

    this.deployment = new QBitTorrentDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      ports: [{ port: 80, targetPort: 3000 }],
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
