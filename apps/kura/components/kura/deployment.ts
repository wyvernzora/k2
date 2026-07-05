import { Size } from "cdk8s";
import {
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

import { K2Deployment, oci, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { KURA_HTTP_PORT, KURA_LABELS, KURA_MCP_PORT } from "../../constants.js";

const KURA_IMAGE = oci`ghcr.io/wyvernzora/kura:v0.3.0`;
const PUID = 3000;
const PGID = 2001;
const UMASK = "0007";
const ANIME_MOUNT_PATH = "/anime";
const LIBRARY_ROOT = `${ANIME_MOUNT_PATH}/series`;
const INBOX_ROOT = `${ANIME_MOUNT_PATH}/downloads`;

export interface KuraDeploymentProps {
  readonly tvdbSecretName: string;
  readonly volumes: K2Volumes;
}

export class KuraDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: KuraDeploymentProps) {
    super(scope, id, {
      metadata: { name: "kura" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: KURA_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
    });

    this.select(LabelSelector.of({ labels: KURA_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    const tvdbSecret = Secret.fromSecretName(this, "tvdb-secret", props.tvdbSecretName);
    this.addInitContainer(initContainer(volumes));
    this.addContainer(kuraContainer(volumes, tvdbSecret));
  }
}

function initContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "init-library",
    image: oci`busybox:1.37.0`,
    command: ["sh", "-c", `set -eu; umask ${UMASK}; mkdir -p ${LIBRARY_ROOT} ${INBOX_ROOT}`],
    volumeMounts: [volumes.anime(ANIME_MOUNT_PATH)],
    securityContext: {
      user: PUID,
      group: PGID,
      ensureNonRoot: true,
    },
  };
}

function kuraContainer(volumes: K2Mounters<K2Volumes>, tvdbSecret: ISecret): ContainerProps {
  const probe = Probe.fromHttpGet("/api/v1/health", { port: KURA_HTTP_PORT });
  return {
    name: "kura",
    image: KURA_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    args: ["serve", `--rest=:${KURA_HTTP_PORT}`, `--mcp-http=:${KURA_MCP_PORT}`],
    ports: [
      { name: "http", number: KURA_HTTP_PORT, protocol: Protocol.TCP },
      { name: "mcp", number: KURA_MCP_PORT, protocol: Protocol.TCP },
    ],
    envVariables: {
      KURA_LIBRARY_ROOT: EnvValue.fromValue(LIBRARY_ROOT),
      KURA_INBOX_ROOT: EnvValue.fromValue(INBOX_ROOT),
      KURA_DISABLE_TOKEN: EnvValue.fromValue("1"),
      KURA_HOST_ID: EnvValue.fromValue("k2-kura"),
      KURA_PREFERRED_LANGUAGES: EnvValue.fromValue("ja"),
      KURA_UMASK: EnvValue.fromValue(UMASK),
      KURA_TVDB_KEY: tvdbSecret.envValue("credential"),
      TZ: EnvValue.fromValue("America/Los_Angeles"),
    },
    volumeMounts: [volumes.anime(ANIME_MOUNT_PATH)],
    liveness: probe,
    readiness: probe,
    resources: {
      cpu: {
        request: Cpu.millis(100),
        limit: Cpu.millis(2000),
      },
      memory: {
        request: Size.mebibytes(256),
        limit: Size.gibibytes(2),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(2),
      },
    },
    securityContext: {
      user: PUID,
      group: PGID,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
  };
}
