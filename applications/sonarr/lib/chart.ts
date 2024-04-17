import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-28";
import { SonarrDeployment, SonarrDeploymentProps } from "./deployment";

export interface SonarrChartProps {
  readonly host: string;
  readonly volumes: SonarrDeploymentProps["volumes"];
}

export class SonarrChart extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: SonarrChartProps) {
    super(scope, id, {});

    this.deployment = new SonarrDeployment(this, "depl", props);
    this.service = this.deployment.exposeViaService({
      ports: [
        {
          port: 80,
          targetPort: 8989,
        },
      ],
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
