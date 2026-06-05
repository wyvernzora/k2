import { Duration, Size } from "cdk8s";
import {
  Capability,
  Cpu,
  Deployment,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Secret,
  Volume,
  type ContainerProps,
  type ISecret,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { Scheduling } from "@k2/cdk-lib";

import { POCKET_ID_HTTP_PORT, POCKET_ID_LABELS } from "../../constants.js";

const POCKET_ID_IMAGE = "ghcr.io/pocket-id/pocket-id:v2.7.0";
const DATA_VOLUME_NAME = "data";
const DATA_MOUNT_PATH = "/app/data";
export const STATIC_API_KEY_SECRET_NAME = "pocket-id-bootstrap-api-key";

export interface PocketIdDeploymentProps {
  readonly appUrl: string;
  readonly credentialsSecretName: string;
  readonly secretName: string;
}

export class PocketIdDeployment extends Deployment {
  public constructor(scope: Construct, id: string, props: PocketIdDeploymentProps) {
    const credentialsSecret = Secret.fromSecretName(scope, `${id}-credentials`, props.credentialsSecretName);
    const pocketIdSecret = Secret.fromSecretName(scope, `${id}-secret`, props.secretName);
    const staticApiKeySecret = Secret.fromSecretName(scope, `${id}-static-api-key`, STATIC_API_KEY_SECRET_NAME);
    const dataVolume = Volume.fromEmptyDir(scope, `${id}-data`, DATA_VOLUME_NAME, { sizeLimit: Size.mebibytes(256) });

    super(scope, id, {
      metadata: { name: "pocket-id" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: POCKET_ID_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: false,
        fsGroup: 1000,
      },
      containers: [container({ ...props, credentialsSecret, dataVolume, pocketIdSecret, staticApiKeySecret })],
    });
    this.select(LabelSelector.of({ labels: POCKET_ID_LABELS }));
    Scheduling.applyWorkersPreferred(this);
  }
}

interface PocketIdContainerProps extends PocketIdDeploymentProps {
  readonly credentialsSecret: ISecret;
  readonly dataVolume: Volume;
  readonly pocketIdSecret: ISecret;
  readonly staticApiKeySecret: ISecret;
}

function container(props: PocketIdContainerProps): ContainerProps {
  return {
    name: "pocket-id",
    image: POCKET_ID_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: POCKET_ID_HTTP_PORT }],
    envVariables: containerEnv(props),
    liveness: execProbe(6),
    readiness: execProbe(3),
    startup: execProbe(30),
    resources: {
      cpu: {
        request: Cpu.millis(50),
        limit: Cpu.millis(500),
      },
      memory: {
        request: Size.mebibytes(128),
        limit: Size.mebibytes(512),
      },
    },
    securityContext: {
      allowPrivilegeEscalation: false,
      capabilities: {
        add: [Capability.CHOWN, Capability.SETGID, Capability.SETUID],
        drop: [Capability.ALL],
      },
      ensureNonRoot: false,
      readOnlyRootFilesystem: false,
    },
    volumeMounts: [{ volume: props.dataVolume, path: DATA_MOUNT_PATH }],
  };
}

function containerEnv(props: PocketIdContainerProps): Record<string, EnvValue> {
  return {
    APP_URL: EnvValue.fromValue(props.appUrl),
    HOST: EnvValue.fromValue("0.0.0.0"),
    PORT: EnvValue.fromValue(String(POCKET_ID_HTTP_PORT)),
    PUID: EnvValue.fromValue("1000"),
    PGID: EnvValue.fromValue("1000"),
    TRUST_PROXY: EnvValue.fromValue("true"),
    FILE_BACKEND: EnvValue.fromValue("database"),
    ANALYTICS_DISABLED: EnvValue.fromValue("true"),
    VERSION_CHECK_DISABLED: EnvValue.fromValue("true"),
    ENCRYPTION_KEY: props.pocketIdSecret.envValue("encryptionKey"),
    STATIC_API_KEY: props.staticApiKeySecret.envValue("apiKey", { optional: true }),
    DB_HOST: props.credentialsSecret.envValue("host"),
    DB_PORT: props.credentialsSecret.envValue("port"),
    DB_NAME: props.credentialsSecret.envValue("dbname"),
    DB_USER: props.credentialsSecret.envValue("user"),
    DB_PASSWORD: props.credentialsSecret.envValue("password"),
    DB_CONNECTION_STRING: EnvValue.fromValue(
      "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable",
    ),
  };
}

function execProbe(failureThreshold: number): Probe {
  return Probe.fromCommand(["/app/pocket-id", "healthcheck"], {
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}
