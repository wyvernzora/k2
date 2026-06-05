import { Duration, Size } from "cdk8s";
import {
  Capability,
  Cpu,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Secret,
  type ContainerProps,
  type ISecret,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

import { K2Deployment, Scheduling, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { PAPERLESS_HTTP_PORT, PAPERLESS_LABELS, PAPERLESS_MCP_PORT } from "../../constants.js";

import { PAPERLESS_SETUP_USER } from "./setup.js";

const PAPERLESS_IMAGE = "ghcr.io/paperless-ngx/paperless-ngx:2.20.15";
const PAPERLESS_MCP_IMAGE = "ghcr.io/baruchiro/paperless-mcp:latest";
const DATA_MOUNT_PATH = "/usr/src/paperless/data";
const DOCUMENTS_MOUNT_PATH = "/paperless-documents";
const MEDIA_MOUNT_PATH = "/usr/src/paperless/media";
const CONSUME_MOUNT_PATH = "/usr/src/paperless/consume";
const EXPORT_MOUNT_PATH = "/usr/src/paperless/export";
const EXPORT_SUBDIR = "exports";
const PAPERLESS_UID = 3003;
const PAPERLESS_GID = 2002;

export interface PaperlessDeploymentProps {
  readonly appUrl: string;
  readonly credentialsSecretName: string;
  readonly mcpTokenSecretName: string;
  readonly secretName: string;
  readonly volumes: K2Volumes;
}

export class PaperlessDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: PaperlessDeploymentProps) {
    super(scope, id, {
      metadata: { name: "paperless" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: PAPERLESS_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: false,
        fsGroup: PAPERLESS_GID,
      },
    });

    this.select(LabelSelector.of({ labels: PAPERLESS_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    const credentialsSecret = Secret.fromSecretName(this, "credentials-secret", props.credentialsSecretName);
    const mcpTokenSecret = Secret.fromSecretName(this, "mcp-token-secret", props.mcpTokenSecretName);
    const paperlessSecret = Secret.fromSecretName(this, "paperless-secret", props.secretName);

    this.addInitContainer(initDocumentsContainer(volumes));
    this.addContainer(paperlessContainer(props, volumes, credentialsSecret, paperlessSecret));
    this.addContainer(paperlessMcpContainer(props.appUrl, mcpTokenSecret));
    Scheduling.applyWorkersPreferred(this);
  }
}

function initDocumentsContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "init-documents",
    image: "busybox:1.37.0",
    command: ["sh", "-c"],
    args: [initDocumentsScript()],
    volumeMounts: [volumes.documents(DOCUMENTS_MOUNT_PATH)],
    securityContext: {
      capabilities: {
        drop: [Capability.ALL],
      },
      user: PAPERLESS_UID,
      group: PAPERLESS_GID,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
  };
}

function paperlessContainer(
  props: PaperlessDeploymentProps,
  volumes: K2Mounters<K2Volumes>,
  credentialsSecret: ISecret,
  paperlessSecret: ISecret,
): ContainerProps {
  const health = paperlessHttpProbe(3);
  return {
    name: "paperless",
    image: PAPERLESS_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: PAPERLESS_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      PAPERLESS_SECRET_KEY: paperlessSecret.envValue("secretKey"),
      PAPERLESS_ADMIN_USER: EnvValue.fromValue(PAPERLESS_SETUP_USER),
      PAPERLESS_ADMIN_PASSWORD: paperlessSecret.envValue("adminPassword"),
      PAPERLESS_DBHOST: credentialsSecret.envValue("host"),
      PAPERLESS_DBPORT: credentialsSecret.envValue("port"),
      PAPERLESS_DBNAME: credentialsSecret.envValue("dbname"),
      PAPERLESS_DBUSER: credentialsSecret.envValue("user"),
      PAPERLESS_DBPASS: credentialsSecret.envValue("password"),
      PAPERLESS_REDIS_PASSWORD: paperlessSecret.envValue("redisPassword"),
      PAPERLESS_REDIS: EnvValue.fromValue("redis://:$(PAPERLESS_REDIS_PASSWORD)@paperless-redis:6379"),
      PAPERLESS_URL: EnvValue.fromValue(props.appUrl),
      PAPERLESS_CSRF_TRUSTED_ORIGINS: EnvValue.fromValue(props.appUrl),
      PAPERLESS_TIME_ZONE: EnvValue.fromValue("America/Los_Angeles"),
      PAPERLESS_OCR_LANGUAGE: EnvValue.fromValue("eng"),
      PAPERLESS_TASK_WORKERS: EnvValue.fromValue("3"),
      PAPERLESS_THREADS_PER_WORKER: EnvValue.fromValue("1"),
      PAPERLESS_CONSUMER_POLLING: EnvValue.fromValue("10"),
      PAPERLESS_ENABLE_HTTP_REMOTE_USER: EnvValue.fromValue("true"),
      PAPERLESS_ENABLE_HTTP_REMOTE_USER_API: EnvValue.fromValue("true"),
      PAPERLESS_HTTP_REMOTE_USER_HEADER_NAME: EnvValue.fromValue("HTTP_X_POMERIUM_CLAIM_PREFERRED_USERNAME"),
      PAPERLESS_DISABLE_REGULAR_LOGIN: EnvValue.fromValue("true"),
      PAPERLESS_ACCOUNT_ALLOW_SIGNUPS: EnvValue.fromValue("false"),
      PAPERLESS_ENABLE_UPDATE_CHECK: EnvValue.fromValue("false"),
      USERMAP_UID: EnvValue.fromValue(String(PAPERLESS_UID)),
      USERMAP_GID: EnvValue.fromValue(String(PAPERLESS_GID)),
    },
    volumeMounts: [
      volumes.data(DATA_MOUNT_PATH),
      volumes.documents(CONSUME_MOUNT_PATH, { subPath: "inbox" }),
      volumes.documents(EXPORT_MOUNT_PATH, { subPath: EXPORT_SUBDIR }),
      volumes.documents(MEDIA_MOUNT_PATH, { subPath: "vault" }),
    ],
    liveness: health,
    readiness: health,
    startup: paperlessHttpProbe(60),
    resources: {
      cpu: {
        request: Cpu.millis(500),
        limit: Cpu.millis(4000),
      },
      memory: {
        request: Size.gibibytes(1),
        limit: Size.gibibytes(6),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(10),
      },
    },
    securityContext: {
      ensureNonRoot: false,
      readOnlyRootFilesystem: false,
    },
  };
}

function paperlessMcpContainer(appUrl: string, mcpTokenSecret: ISecret): ContainerProps {
  return {
    name: "paperless-mcp",
    image: PAPERLESS_MCP_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "mcp", number: PAPERLESS_MCP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      PAPERLESS_URL: EnvValue.fromValue(`http://127.0.0.1:${PAPERLESS_HTTP_PORT}`),
      PAPERLESS_PUBLIC_URL: EnvValue.fromValue(appUrl),
      PAPERLESS_API_KEY: mcpTokenSecret.envValue("apiKey"),
      PAPERLESS_API_VERSION: EnvValue.fromValue("9"),
    },
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(500),
      },
      memory: {
        request: Size.mebibytes(128),
        limit: Size.mebibytes(512),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      allowPrivilegeEscalation: false,
      capabilities: { drop: [Capability.ALL] },
      ensureNonRoot: false,
      readOnlyRootFilesystem: true,
    },
  };
}

function paperlessHttpProbe(failureThreshold: number): Probe {
  return Probe.fromHttpGet("/", {
    port: PAPERLESS_HTTP_PORT,
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}

function initDocumentsScript(): string {
  return dedent`
    set -eu
    umask 0007

    mkdir -p ${DOCUMENTS_MOUNT_PATH}/inbox
    mkdir -p ${DOCUMENTS_MOUNT_PATH}/${EXPORT_SUBDIR}
    mkdir -p ${DOCUMENTS_MOUNT_PATH}/vault
    chmod 2770 ${DOCUMENTS_MOUNT_PATH}/inbox
    chmod 2770 ${DOCUMENTS_MOUNT_PATH}/${EXPORT_SUBDIR}
    chmod 2770 ${DOCUMENTS_MOUNT_PATH}/vault
  `;
}
