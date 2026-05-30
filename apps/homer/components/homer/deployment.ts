import { Size } from "cdk8s";
import {
  ConfigMap,
  Cpu,
  Deployment,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Protocol,
  Volume,
  type ContainerProps,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { Scheduling } from "@k2/cdk-lib";

import { HOMER_HTTP_PORT, HOMER_LABELS } from "./labels.js";

const HOMER_IMAGE = "b4bz/homer:v26.4.2";
const CONFIG_KEY = "config.yml";
const CONFIG_VOLUME_NAME = "config";
const CONFIG_MOUNT_PATH = "/www/assets/config.yml";

export interface HomerDeploymentProps {
  readonly configName: string;
  readonly configChecksum: string;
}

export class HomerDeployment extends Deployment {
  public constructor(scope: Construct, id: string, props: HomerDeploymentProps) {
    const config = ConfigMap.fromConfigMapName(scope, `${id}-config`, props.configName);
    const configVolume = Volume.fromConfigMap(scope, `${id}-config-volume`, config, { name: CONFIG_VOLUME_NAME });

    super(scope, id, {
      metadata: { name: "homer" },
      replicas: 1,
      select: false,
      podMetadata: {
        labels: HOMER_LABELS,
        annotations: {
          "checksum/homer-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
      containers: [homerContainer(configVolume)],
      volumes: [configVolume],
    });
    this.select(LabelSelector.of({ labels: HOMER_LABELS }));
    Scheduling.applyWorkersPreferred(this);
  }
}

function homerContainer(configVolume: Volume): ContainerProps {
  return {
    name: "homer",
    image: HOMER_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: HOMER_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      INIT_ASSETS: EnvValue.fromValue("0"),
      PORT: EnvValue.fromValue(String(HOMER_HTTP_PORT)),
    },
    volumeMounts: [
      {
        volume: configVolume,
        path: CONFIG_MOUNT_PATH,
        subPath: CONFIG_KEY,
      },
    ],
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(250),
      },
      memory: {
        request: Size.mebibytes(64),
        limit: Size.mebibytes(256),
      },
    },
    securityContext: {
      allowPrivilegeEscalation: false,
      ensureNonRoot: true,
      readOnlyRootFilesystem: false,
    },
  };
}
