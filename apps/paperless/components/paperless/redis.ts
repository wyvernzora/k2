import { Cpu, Deployment, EnvValue, ISecret, Probe } from "cdk8s-plus-32";
import { Construct } from "constructs";
import { Size } from "cdk8s";

import { oci } from "@k2/cdk-lib";

export interface RedisDeploymentProps {
  readonly secret: ISecret;
}

export class RedisDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: RedisDeploymentProps) {
    super(scope, id, {
      replicas: 1,
    });

    this.addContainer({
      name: "redis",
      image: oci`redis:7.4-alpine`,
      args: ["redis-server", "--save", "", "--appendonly", "no", "--requirepass", "$(REDIS_PASSWORD)"],
      ports: [
        {
          name: "redis",
          number: 6379,
        },
      ],
      envVariables: {
        REDIS_PASSWORD: EnvValue.fromSecretValue({
          secret: props.secret,
          key: "redisPassword",
        }),
      },
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
      resources: {
        cpu: {
          request: Cpu.millis(25),
          limit: Cpu.millis(500),
        },
        memory: {
          request: Size.mebibytes(64),
          limit: Size.mebibytes(256),
        },
      },
      liveness: Probe.fromCommand(["sh", "-c", 'redis-cli -a "$REDIS_PASSWORD" ping']),
      readiness: Probe.fromCommand(["sh", "-c", 'redis-cli -a "$REDIS_PASSWORD" ping']),
    });
  }
}
