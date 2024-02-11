import { QbitTorrentDeployment, QbitTorrentVolumes } from "./deployment";
import { Chart } from "cdk8s";
import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-27";
import { Construct } from "constructs";

export interface QbitTorrentChartProps {
  readonly host: string;
  readonly volumes: QbitTorrentVolumes;
}

export class QbitTorrentChart extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: QbitTorrentChartProps) {
    super(scope, id, {});

    this.deployment = new QbitTorrentDeployment(this, "depl", props);
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
