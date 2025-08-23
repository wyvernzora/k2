import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-28";
import { Construct } from "constructs";

import { AuthenticatedIngress } from "@k2/auth";

import { QBitTorrentDeployment, QBitTorrentDeploymentProps } from "./deployment.js";

export interface QBitTorrentProps {
  readonly host: string;
  readonly volumes: QBitTorrentDeploymentProps["volumes"];
  readonly downloads: QBitTorrentDeploymentProps["downloads"];
}

export class QBitTorrent extends Construct {
  readonly deployment: Deployment;
  readonly floodService: Service;
  readonly qbittorrentService: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: QBitTorrentProps) {
    super(scope, id);

    this.deployment = new QBitTorrentDeployment(this, "depl", props);
    this.floodService = this.deployment.exposeViaService({
      name: "flood",
      ports: [{ port: 80, targetPort: 3000 }],
    });
    this.qbittorrentService = this.deployment.exposeViaService({
      name: "qbittorrent",
      ports: [{ port: 80, targetPort: 8080 }],
    });
    this.ingress = new AuthenticatedIngress(this, "ingr", {
      rules: [
        {
          host: props.host,
          backend: IngressBackend.fromService(this.floodService),
        },
      ],
    });
  }
}
