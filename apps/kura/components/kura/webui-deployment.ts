import { Size } from "cdk8s";
import {
  Capability,
  Cpu,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  SeccompProfileType,
  Volume,
  type ContainerProps,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment, oci } from "@k2/cdk-lib";

import { KURA_SERVICE_NAME, KURA_WEBUI_HTTP_PORT, KURA_WEBUI_LABELS } from "../../constants.js";

const KURA_WEBUI_IMAGE = oci`ghcr.io/wyvernzora/kura/webui:v0.6.1`;
const CADDY_UID = 65532;
const CADDY_GID = 65532;

export class KuraWebuiDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string) {
    const volumeMounts = caddyVolumeMounts(scope, id);
    super(scope, id, {
      metadata: { name: "kura-webui" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.rollingUpdate(),
      podMetadata: { labels: KURA_WEBUI_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
        fsGroup: CADDY_GID,
      },
      volumes: volumeMounts.map(mount => mount.volume),
      containers: [webuiContainer(volumeMounts)],
    });
    this.select(LabelSelector.of({ labels: KURA_WEBUI_LABELS }));
  }
}

function webuiContainer(volumeMounts: VolumeMount[]): ContainerProps {
  const probe = Probe.fromHttpGet("/", { port: KURA_WEBUI_HTTP_PORT });
  return {
    name: "webui",
    image: KURA_WEBUI_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    ports: [{ name: "http", number: KURA_WEBUI_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      KURA_WEBUI_LIBRARY_UPSTREAM: EnvValue.fromValue(`http://${KURA_SERVICE_NAME}:80`),
      XDG_CONFIG_HOME: EnvValue.fromValue("/config"),
      XDG_DATA_HOME: EnvValue.fromValue("/data"),
    },
    volumeMounts,
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
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      user: CADDY_UID,
      group: CADDY_GID,
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

function caddyVolumeMounts(scope: Construct, id: string): VolumeMount[] {
  return [
    emptyDirMount(scope, `${id}-config`, "caddy-config", "/config"),
    emptyDirMount(scope, `${id}-data`, "caddy-data", "/data"),
    emptyDirMount(scope, `${id}-tmp`, "caddy-tmp", "/tmp"),
  ];
}

function emptyDirMount(scope: Construct, id: string, name: string, path: string): VolumeMount {
  return {
    volume: Volume.fromEmptyDir(scope, id, name, {
      sizeLimit: Size.gibibytes(1),
    }),
    path,
  };
}
