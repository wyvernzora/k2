import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, Ingress, IngressBackend, Service } from "cdk8s-plus-32";

import { AuthenticatedIngress } from "@k2/auth";
import { VolumesOf } from "@k2/cdk-lib";
import { K2Secret } from "@k2/1password";

import { PaperlessDatabase } from "./database.js";
import { PaperlessDeployment, PaperlessDeploymentProps } from "./deployment.js";
import { RedisDeployment } from "./redis.js";

export interface PaperlessProps {
  readonly host: string;
  readonly volumes: VolumesOf<PaperlessDeploymentProps>;
}

export class Paperless extends Chart {
  readonly database: PaperlessDatabase;
  readonly deployment: Deployment;
  readonly redis: Deployment;
  readonly service: Service;
  readonly redisService: Service;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: PaperlessProps) {
    super(scope, id, {});

    const secret = new K2Secret(this, "secret", {
      metadata: {
        name: "paperless",
      },
      itemId: "pw2aod2wbvjibgkkm552mz6jfu",
    });

    this.database = new PaperlessDatabase(this, "database");
    this.redis = new RedisDeployment(this, "redis", {
      secret: secret.secret,
    });
    this.redisService = this.redis.exposeViaService({
      name: "paperless-redis",
      ports: [{ port: 6379, targetPort: 6379 }],
    });
    this.deployment = new PaperlessDeployment(this, "depl", {
      database: this.database,
      redisHost: this.redisService.name,
      secret: secret.secret,
      volumes: props.volumes,
    });
    this.service = this.deployment.exposeViaService({
      name: "paperless",
      ports: [{ port: 80, targetPort: 8000 }],
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
