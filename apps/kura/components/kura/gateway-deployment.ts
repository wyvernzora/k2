import { Size } from "cdk8s";
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
  SeccompProfileType,
  Volume,
  type ContainerProps,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment } from "@k2/cdk-lib";

import {
  KURA_GATEWAY_HTTP_PORT,
  KURA_GATEWAY_LABELS,
  KURA_GATEWAY_MCP_METRICS_PORT,
  KURA_GATEWAY_METRICS_PORT,
  KURA_LIBRARY_MANAGER_HTTP_PORT,
  KURA_LIBRARY_MANAGER_SERVICE_NAME,
  KURA_RELEASE_INDEXER_HTTP_PORT,
  KURA_RELEASE_INDEXER_SERVICE_NAME,
} from "../../constants.js";
import { KURA_IMAGES } from "../../images.js";

import { GATEWAY_CONFIG_KEY } from "./config.js";

const CADDY_UID = 65532;
const CADDY_GID = 65532;
const CONFIG_MOUNT_PATH = "/etc/kura/gateway.toml";

export interface KuraGatewayDeploymentProps {
  readonly configChecksum: string;
  readonly configName: string;
}

export class KuraGatewayDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: KuraGatewayDeploymentProps) {
    const volumeMounts = caddyVolumeMounts(scope, id);
    const config = ConfigMap.fromConfigMapName(scope, `${id}-gateway-config`, props.configName);
    const configVolume = Volume.fromConfigMap(scope, `${id}-config-volume`, config, { name: "config" });
    super(scope, id, {
      metadata: { name: "kura-gateway" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.rollingUpdate(),
      podMetadata: {
        labels: KURA_GATEWAY_LABELS,
        annotations: {
          "checksum/gateway-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
        fsGroup: CADDY_GID,
      },
      volumes: [configVolume, ...volumeMounts.map(mount => mount.volume)],
      containers: [gatewayContainer(volumeMounts, configVolume)],
    });
    this.select(LabelSelector.of({ labels: KURA_GATEWAY_LABELS }));
  }
}

function gatewayContainer(volumeMounts: VolumeMount[], configVolume: Volume): ContainerProps {
  const probe = Probe.fromHttpGet("/healthz", { port: KURA_GATEWAY_HTTP_PORT });
  return {
    name: "gateway",
    image: KURA_IMAGES.gateway,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    ports: [
      { name: "http", number: KURA_GATEWAY_HTTP_PORT, protocol: Protocol.TCP },
      { name: "metrics", number: KURA_GATEWAY_METRICS_PORT, protocol: Protocol.TCP },
      { name: "mcp-metrics", number: KURA_GATEWAY_MCP_METRICS_PORT, protocol: Protocol.TCP },
    ],
    envVariables: {
      KURA_LIBRARY_UPSTREAM: EnvValue.fromValue(
        `${KURA_LIBRARY_MANAGER_SERVICE_NAME}:${KURA_LIBRARY_MANAGER_HTTP_PORT}`,
      ),
      KURA_RELEASES_UPSTREAM: EnvValue.fromValue(
        `${KURA_RELEASE_INDEXER_SERVICE_NAME}:${KURA_RELEASE_INDEXER_HTTP_PORT}`,
      ),
      XDG_CONFIG_HOME: EnvValue.fromValue("/config"),
      XDG_DATA_HOME: EnvValue.fromValue("/data"),
    },
    volumeMounts: [...volumeMounts, configMount(configVolume)],
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

function configMount(configVolume: Volume): VolumeMount {
  return {
    volume: configVolume,
    path: CONFIG_MOUNT_PATH,
    subPath: GATEWAY_CONFIG_KEY,
    readOnly: true,
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
