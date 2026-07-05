import { Size } from "cdk8s";
import {
  ConfigMap,
  Cpu,
  DaemonSet,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Protocol,
  Volume,
  type ContainerProps,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { oci, prefer, Scheduling, workers } from "@k2/cdk-lib";

const BLOCKY_IMAGE = oci`ghcr.io/0xerr0r/blocky:v0.33.0`;
const CONFIG_KEY = "blocky.yaml";
const CONFIG_VOLUME_NAME = "config";
const CONFIG_MOUNT_PATH = "/app/config.yml";
const POD_LABELS = {
  "app.kubernetes.io/name": "blocky",
  "app.kubernetes.io/component": "resolver",
};

export interface BlockyDaemonSetProps {
  readonly configName: string;
  readonly configChecksum: string;
}

export class BlockyDaemonSet extends DaemonSet {
  public readonly selectorLabels = { ...POD_LABELS };

  public constructor(scope: Construct, id: string, props: BlockyDaemonSetProps) {
    const config = ConfigMap.fromConfigMapName(scope, `${id}-config`, props.configName);
    const configVolume = Volume.fromConfigMap(scope, `${id}-config-volume`, config, { name: CONFIG_VOLUME_NAME });

    super(scope, id, {
      metadata: { name: "blocky" },
      select: false,
      podMetadata: {
        labels: { ...POD_LABELS },
        annotations: {
          "checksum/blocky-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: { ensureNonRoot: false },
      containers: [blockyContainer(configVolume)],
      volumes: [configVolume],
    });
    this.select(LabelSelector.of({ labels: POD_LABELS }));
    Scheduling.of(this).apply(prefer(workers()));
  }
}

function blockyContainer(configVolume: Volume): ContainerProps {
  return {
    name: "blocky",
    image: BLOCKY_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [
      { name: "dns-udp", number: 53, protocol: Protocol.UDP },
      { name: "dns-tcp", number: 53, protocol: Protocol.TCP },
      { name: "http", number: 4000, protocol: Protocol.TCP },
    ],
    envVariables: { TZ: EnvValue.fromValue("America/Los_Angeles") },
    volumeMounts: [
      {
        volume: configVolume,
        path: CONFIG_MOUNT_PATH,
        subPath: CONFIG_KEY,
      },
    ],
    resources: {
      cpu: {
        request: Cpu.millis(100),
        limit: Cpu.millis(250),
      },
      memory: {
        request: Size.mebibytes(256),
        limit: Size.mebibytes(1024),
      },
    },
    securityContext: {
      ensureNonRoot: false,
      readOnlyRootFilesystem: false,
    },
  };
}
