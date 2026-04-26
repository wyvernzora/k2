import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, HttpIngressPathType, Ingress, IngressBackend, Service } from "cdk8s-plus-32";

import { K2Secret } from "@k2/1password";
import { AuthenticatedIngress } from "@k2/auth";
import { ApexDomain, Namespace, VolumesOf } from "@k2/cdk-lib";

import { OpenClawConfig } from "./config.js";
import { OpenClawDeployment, OpenClawDeploymentProps } from "./deployment.js";

export interface OpenClawProps {
  readonly volumes: VolumesOf<OpenClawDeploymentProps>;
}

export class OpenClaw extends Chart {
  readonly config: OpenClawConfig;
  readonly deployment: Deployment;
  readonly service: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: OpenClawProps) {
    super(scope, id, {
      ...Namespace.of(scope),
    });

    const hostname = ApexDomain.of(this).subdomain("claw");
    this.config = new OpenClawConfig(this, "config", {
      allowedOrigin: `https://${hostname}`,
    });
    const openAiCredentials = new K2Secret(this, "openai-credentials", {
      metadata: {
        name: "openclaw-openai-credentials",
      },
      itemId: "wpysnhydomdyqie3shifuxpgly",
    });
    this.deployment = new OpenClawDeployment(this, "depl", {
      config: this.config,
      openAiSecret: openAiCredentials.secret,
      volumes: props.volumes,
    });
    this.service = this.deployment.exposeViaService({
      name: "openclaw",
      ports: [{ port: 80, targetPort: 18789 }],
    });
    this.ingress = new AuthenticatedIngress(this, "ingress", {
      rules: [
        {
          host: hostname,
          pathType: HttpIngressPathType.PREFIX,
          backend: IngressBackend.fromService(this.service),
        },
      ],
    });
  }
}
