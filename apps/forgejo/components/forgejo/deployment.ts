import { ApiObject, Duration, JsonPatch, Size } from "cdk8s";
import {
  Capability,
  ConfigMap,
  Cpu,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Secret,
  SeccompProfileType,
  Volume,
  type ContainerProps,
  type ISecret,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

import { K2Deployment, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { FORGEJO_HTTP_PORT, FORGEJO_HTTPS_PORT, FORGEJO_LABELS, FORGEJO_SSH_PORT } from "../../constants.js";

import { forgejoEnv } from "./env.js";

export const FORGEJO_IMAGE = "codeberg.org/forgejo/forgejo:15-rootless";
export const FORGEJO_APPDATA_CLAIM_NAME = "forgejo-appdata";

const CADDY_IMAGE = "caddy:2.10.0-alpine";
const BUSYBOX_IMAGE = "busybox:1.37.0";
const DEFAULT_CERTIFICATE_SECRET_NAME = "default-certificate";
const PUID = 1000;
const PGID = 1000;
const APPDATA_INIT_PATH = "/forgejo";
export const APPDATA_MOUNT_PATH = "/var/lib/gitea";
export const CONFIG_MOUNT_PATH = "/etc/gitea";
export const CONFIG_FILE_PATH = `${APPDATA_MOUNT_PATH}/custom/conf/app.ini`;

export interface ForgejoDeploymentProps {
  readonly credentialsSecretName: string;
  readonly secretName: string;
  readonly volumes: K2Volumes;
}

export class ForgejoDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: ForgejoDeploymentProps) {
    super(scope, id, {
      metadata: { name: "forgejo" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: FORGEJO_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        fsGroup: PGID,
        ensureNonRoot: true,
      },
    });

    this.select(LabelSelector.of({ labels: FORGEJO_LABELS }));
    ApiObject.of(this).addJsonPatch(JsonPatch.remove("/spec/template/spec/securityContext/fsGroupChangePolicy"));
    const volumes = this.attachVolumes(props.volumes);
    const credentialsSecret = Secret.fromSecretName(this, "credentials-secret", props.credentialsSecretName);
    const forgejoSecret = Secret.fromSecretName(this, "forgejo-secret", props.secretName);
    const caddyMounts = caddyVolumeMounts(this);

    this.addInitContainer(initAppdataContainer(volumes));
    this.addContainer(forgejoContainer(volumes, credentialsSecret, forgejoSecret));
    this.addContainer(caddyContainer(caddyMounts));
  }
}

function initAppdataContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "init-forgejo-appdata",
    image: BUSYBOX_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: ["sh", "-c", initAppdataScript()],
    volumeMounts: [volumes.appdata(APPDATA_INIT_PATH)],
    resources: initResources(),
    securityContext: {
      user: 0,
      group: 0,
      ensureNonRoot: false,
    },
  };
}

function forgejoContainer(
  volumes: K2Mounters<K2Volumes>,
  credentialsSecret: ISecret,
  forgejoSecret: ISecret,
): ContainerProps {
  return {
    name: "forgejo",
    image: FORGEJO_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [
      { name: "http", number: FORGEJO_HTTP_PORT, protocol: Protocol.TCP },
      { name: "ssh", number: FORGEJO_SSH_PORT, protocol: Protocol.TCP },
    ],
    envVariables: forgejoEnv({ credentialsSecret, forgejoSecret }),
    volumeMounts: [
      volumes.appdata(APPDATA_MOUNT_PATH, { subPath: "data" }),
      volumes.appdata(CONFIG_MOUNT_PATH, { subPath: "config" }),
    ],
    liveness: forgejoHttpProbe(6),
    readiness: forgejoHttpProbe(3),
    startup: forgejoHttpProbe(60),
    resources: {
      cpu: {
        request: Cpu.millis(250),
        limit: Cpu.millis(2000),
      },
      memory: {
        request: Size.mebibytes(512),
        limit: Size.gibibytes(4),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(8),
      },
    },
    securityContext: {
      user: PUID,
      group: PGID,
      allowPrivilegeEscalation: false,
      capabilities: {
        drop: [Capability.ALL],
      },
      ensureNonRoot: true,
      readOnlyRootFilesystem: false,
      seccompProfile: {
        type: SeccompProfileType.RUNTIME_DEFAULT,
      },
    },
  };
}

function caddyContainer(volumeMounts: VolumeMount[]): ContainerProps {
  return {
    name: "caddy",
    image: CADDY_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "https", number: FORGEJO_HTTPS_PORT, protocol: Protocol.TCP }],
    volumeMounts,
    envVariables: {
      XDG_CONFIG_HOME: EnvValue.fromValue("/config"),
      XDG_DATA_HOME: EnvValue.fromValue("/data"),
    },
    liveness: caddyTcpProbe(6),
    readiness: caddyTcpProbe(3),
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(500),
      },
      memory: {
        request: Size.mebibytes(64),
        limit: Size.mebibytes(512),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      user: 65532,
      group: 65532,
      allowPrivilegeEscalation: false,
      capabilities: {
        add: [Capability.NET_BIND_SERVICE],
        drop: [Capability.ALL],
      },
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
      seccompProfile: {
        type: SeccompProfileType.RUNTIME_DEFAULT,
      },
    },
  };
}

function initResources(): ContainerProps["resources"] {
  return {
    cpu: {
      request: Cpu.millis(25),
      limit: Cpu.millis(500),
    },
    memory: {
      request: Size.mebibytes(32),
      limit: Size.mebibytes(256),
    },
    ephemeralStorage: {
      limit: Size.gibibytes(1),
    },
  };
}

function forgejoHttpProbe(failureThreshold: number): Probe {
  return Probe.fromHttpGet("/api/healthz", {
    port: FORGEJO_HTTP_PORT,
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}

function caddyTcpProbe(failureThreshold: number): Probe {
  return Probe.fromTcpSocket({
    port: FORGEJO_HTTPS_PORT,
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}

function caddyVolumeMounts(scope: Construct): VolumeMount[] {
  const configMap = new ConfigMap(scope, "caddy-config", {
    data: {
      Caddyfile: caddyfile(),
    },
  });
  const certificate = Secret.fromSecretName(scope, "default-certificate", DEFAULT_CERTIFICATE_SECRET_NAME);
  return [
    {
      volume: Volume.fromConfigMap(scope, "caddy-config-volume", configMap),
      path: "/etc/caddy/Caddyfile",
      subPath: "Caddyfile",
    },
    {
      volume: Volume.fromSecret(scope, "default-certificate-volume", certificate),
      path: "/tls",
      readOnly: true,
    },
    {
      volume: Volume.fromEmptyDir(scope, "caddy-config-state-volume", "caddy-config-state", {
        sizeLimit: Size.gibibytes(1),
      }),
      path: "/config",
    },
    {
      volume: Volume.fromEmptyDir(scope, "caddy-data-volume", "caddy-data", {
        sizeLimit: Size.gibibytes(1),
      }),
      path: "/data",
    },
    {
      volume: Volume.fromEmptyDir(scope, "caddy-tmp-volume", "caddy-tmp", {
        sizeLimit: Size.gibibytes(1),
      }),
      path: "/tmp",
    },
  ];
}

function initAppdataScript(): string {
  return dedent`
    set -eu

    mkdir -p "${APPDATA_INIT_PATH}/data" "${APPDATA_INIT_PATH}/config"
    chown -R ${PUID}:${PGID} "${APPDATA_INIT_PATH}/data" "${APPDATA_INIT_PATH}/config"
  `;
}

function caddyfile(): string {
  return dedent`
    {
      auto_https off
    }

    :${FORGEJO_HTTPS_PORT} {
      tls /tls/tls.crt /tls/tls.key
      reverse_proxy 127.0.0.1:${FORGEJO_HTTP_PORT}
    }
  `;
}
