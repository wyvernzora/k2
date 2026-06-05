import { Size } from "cdk8s";
import {
  Cpu,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Secret,
  type ContainerProps,
  type ISecret,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment } from "@k2/cdk-lib";

import { REDIS_LABELS, REDIS_PORT } from "../../constants.js";

const REDIS_IMAGE = "redis:7.4-alpine";

export interface RedisDeploymentProps {
  readonly secretName: string;
}

export class RedisDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: RedisDeploymentProps) {
    super(scope, id, {
      metadata: { name: "paperless-redis" },
      replicas: 1,
      select: false,
      podMetadata: { labels: REDIS_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
    });

    this.select(LabelSelector.of({ labels: REDIS_LABELS }));
    const secret = Secret.fromSecretName(this, "paperless-secret", props.secretName);
    this.addContainer(redisContainer(secret));
  }
}

function redisContainer(secret: ISecret): ContainerProps {
  const probe = Probe.fromCommand(["sh", "-c", 'redis-cli -a "$REDIS_PASSWORD" ping']);
  return {
    name: "redis",
    image: REDIS_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    args: ["redis-server", "--save", "", "--appendonly", "no", "--requirepass", "$(REDIS_PASSWORD)"],
    ports: [{ name: "redis", number: REDIS_PORT, protocol: Protocol.TCP }],
    envVariables: {
      REDIS_PASSWORD: secret.envValue("redisPassword"),
    },
    liveness: probe,
    readiness: probe,
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
    securityContext: {
      ensureNonRoot: false,
      readOnlyRootFilesystem: false,
    },
  };
}
