import {
  QBitTorrentDeployment,
  QBitTorrentDeploymentProps,
} from "./deployment";
import { Chart } from "cdk8s";
import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-28";
import { Construct } from "constructs";

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
    this.ingress = new Ingress(this, "ingress", {
      metadata: {
        annotations: {
          "traefik.ingress.kubernetes.io/router.middlewares":
            "k2-auth-authelia@kubernetescrd",
        },
      },
      rules: [
        {
          host: props.host,
          backend: IngressBackend.fromService(this.service),
        },
      ],
    });
  }
}
