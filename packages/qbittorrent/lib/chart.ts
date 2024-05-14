import {
  QBitTorrentDeployment,
  QBitTorrentDeploymentProps,
} from "./deployment";
import { Chart } from "cdk8s";
import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { AuthenticatedIngress } from "@k2/authelia";

export interface QBitTorrentChartProps {
  readonly host: string;
  readonly volumes: QBitTorrentDeploymentProps["volumes"];
}

export class QBitTorrentChart extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: QBitTorrentChartProps) {
    super(scope, id, {});

    this.deployment = new QBitTorrentDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      ports: [{ port: 80, targetPort: 8080 }],
    });
    this.ingress = new AuthenticatedIngress(this, "ingress", {
      rules: [
        {
          host: props.host,
          backend: IngressBackend.fromService(this.service),
        },
      ],
    });
  }
}
