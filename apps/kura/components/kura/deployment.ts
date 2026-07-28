import { Size } from "cdk8s";
import {
  ConfigMap,
  Cpu,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Secret,
  Volume,
  type ContainerProps,
  type ISecret,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment, oci, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { KURA_LIBRARY_MANAGER_HTTP_PORT, KURA_LIBRARY_MANAGER_LABELS } from "../../constants.js";

import { LIBRARY_MANAGER_CONFIG_KEY } from "./config.js";

const KURA_IMAGE = oci`ghcr.io/wyvernzora/kura/library-manager:v0.7.0`;
const PUID = 3000;
const PGID = 2001;
const UMASK = "0007";
const ANIME_MOUNT_PATH = "/anime";
const LIBRARY_ROOT = `${ANIME_MOUNT_PATH}/series`;
const INBOX_ROOT = `${ANIME_MOUNT_PATH}/downloads`;
const CONFIG_MOUNT_PATH = "/etc/kura/library-manager.toml";

export interface KuraDeploymentProps {
  readonly configChecksum: string;
  readonly configName: string;
  readonly tvdbSecretName: string;
  readonly volumes: K2Volumes;
}

export class KuraDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: KuraDeploymentProps) {
    const config = ConfigMap.fromConfigMapName(scope, `${id}-config`, props.configName);
    const configVolume = Volume.fromConfigMap(scope, `${id}-config-volume`, config, { name: "config" });
    super(scope, id, {
      metadata: { name: "kura-library-manager" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: {
        labels: KURA_LIBRARY_MANAGER_LABELS,
        annotations: {
          "checksum/library-manager-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
      volumes: [configVolume],
    });

    this.select(LabelSelector.of({ labels: KURA_LIBRARY_MANAGER_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    const tvdbSecret = Secret.fromSecretName(this, "tvdb-secret", props.tvdbSecretName);
    this.addInitContainer(initContainer(volumes));
    this.addContainer(kuraContainer(volumes, configVolume, tvdbSecret));
  }
}

function initContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "init-library",
    image: oci`busybox:1.38.0`,
    command: ["sh", "-c", `set -eu; umask ${UMASK}; mkdir -p ${LIBRARY_ROOT} ${INBOX_ROOT}`],
    volumeMounts: [volumes.anime(ANIME_MOUNT_PATH)],
    securityContext: {
      user: PUID,
      group: PGID,
      ensureNonRoot: true,
    },
  };
}

function kuraContainer(volumes: K2Mounters<K2Volumes>, configVolume: Volume, tvdbSecret: ISecret): ContainerProps {
  const probe = Probe.fromHttpGet("/healthz", { port: KURA_LIBRARY_MANAGER_HTTP_PORT });
  return {
    name: "library-manager",
    image: KURA_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    args: [`--config=${CONFIG_MOUNT_PATH}`],
    ports: [{ name: "http", number: KURA_LIBRARY_MANAGER_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      KURA_HOST_ID: EnvValue.fromValue("k2-kura"),
      KURA_TVDB_KEY: tvdbSecret.envValue("credential"),
      TZ: EnvValue.fromValue("America/Los_Angeles"),
    },
    volumeMounts: [volumes.anime(ANIME_MOUNT_PATH), configMount(configVolume)],
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

function configMount(configVolume: Volume): VolumeMount {
  return {
    volume: configVolume,
    path: CONFIG_MOUNT_PATH,
    subPath: LIBRARY_MANAGER_CONFIG_KEY,
    readOnly: true,
  };
}
