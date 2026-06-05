import { Size } from "cdk8s";
import {
  ConfigMap,
  Cpu,
  DeploymentStrategy,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Volume,
  type ContainerProps,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { MOSQUITTO_LABELS, MOSQUITTO_MQTT_PORT } from "../../constants.js";

const MOSQUITTO_IMAGE = "eclipse-mosquitto:2.0.22";

export interface MosquittoDeploymentProps {
  readonly configChecksum: string;
  readonly configName: string;
  readonly volumes: K2Volumes;
}

export class MosquittoDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: MosquittoDeploymentProps) {
    super(scope, id, {
      metadata: { name: "mosquitto" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: {
        labels: MOSQUITTO_LABELS,
        annotations: {
          "checksum/mosquitto-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: false,
      },
    });

    this.select(LabelSelector.of({ labels: MOSQUITTO_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    this.addContainer(mosquittoContainer(volumes, configMount(this, props.configName)));
  }
}

function mosquittoContainer(volumes: K2Mounters<K2Volumes>, configMount: VolumeMount): ContainerProps {
  return {
    name: "mosquitto",
    image: MOSQUITTO_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "mqtt", number: MOSQUITTO_MQTT_PORT, protocol: Protocol.TCP }],
    volumeMounts: [volumes.data("/mosquitto/data"), volumes.logs("/mosquitto/log"), configMount],
    liveness: Probe.fromCommand(["mosquitto_sub", "-t", "$$SYS/#", "-C", "1", "-W", "5"]),
    readiness: Probe.fromCommand(["mosquitto_sub", "-t", "$$SYS/#", "-C", "1", "-W", "5"]),
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
      ensureNonRoot: false,
      readOnlyRootFilesystem: false,
    },
  };
}

function configMount(scope: Construct, configName: string): VolumeMount {
  const config = ConfigMap.fromConfigMapName(scope, "mosquitto-config", configName);
  return {
    volume: Volume.fromConfigMap(scope, "mosquitto-config-volume", config),
    path: "/mosquitto/config/mosquitto.conf",
    subPath: "mosquitto.conf",
  };
}
