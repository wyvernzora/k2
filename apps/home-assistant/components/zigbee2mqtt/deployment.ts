import { Duration, Size } from "cdk8s";
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

import { K2Deployment, Scheduling, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { ZIGBEE2MQTT_HTTP_PORT, ZIGBEE2MQTT_LABELS } from "../../constants.js";

const ZIGBEE2MQTT_IMAGE = "koenkk/zigbee2mqtt:2.10.1";
const DATA_MOUNT_PATH = "/app/data";
const INIT_MOUNT_PATH = "/init";

export interface Zigbee2MqttDeploymentProps {
  readonly configChecksum: string;
  readonly configName: string;
  readonly volumes: K2Volumes;
}

export class Zigbee2MqttDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: Zigbee2MqttDeploymentProps) {
    super(scope, id, {
      metadata: { name: "zigbee2mqtt" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: {
        labels: ZIGBEE2MQTT_LABELS,
        annotations: {
          "checksum/zigbee2mqtt-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: false,
      },
    });

    this.select(LabelSelector.of({ labels: ZIGBEE2MQTT_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    const initMount = initConfigMount(this, props.configName);
    this.addInitContainer(initConfigContainer(volumes, initMount));
    this.addContainer(zigbee2MqttContainer(volumes));
    Scheduling.applyWorkersPreferred(this);
  }
}

function initConfigContainer(volumes: K2Mounters<K2Volumes>, initMount: VolumeMount): ContainerProps {
  return {
    name: "setup-config",
    image: "busybox:1.37.0",
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: ["sh", `${INIT_MOUNT_PATH}/init.sh`],
    volumeMounts: [volumes.data(DATA_MOUNT_PATH), initMount],
    resources: initResources(),
    securityContext: {
      ensureNonRoot: false,
      readOnlyRootFilesystem: true,
    },
  };
}

function zigbee2MqttContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  const health = zigbee2MqttProbe(3);
  return {
    name: "zigbee2mqtt",
    image: ZIGBEE2MQTT_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: ZIGBEE2MQTT_HTTP_PORT, protocol: Protocol.TCP }],
    volumeMounts: [volumes.data(DATA_MOUNT_PATH)],
    liveness: health,
    readiness: health,
    startup: zigbee2MqttProbe(60),
    resources: {
      cpu: {
        request: Cpu.millis(50),
        limit: Cpu.millis(1000),
      },
      memory: {
        request: Size.mebibytes(256),
        limit: Size.gibibytes(1),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(2),
      },
    },
    securityContext: {
      ensureNonRoot: false,
      readOnlyRootFilesystem: false,
    },
  };
}

function initResources(): ContainerProps["resources"] {
  return {
    cpu: {
      request: Cpu.millis(10),
      limit: Cpu.millis(250),
    },
    memory: {
      request: Size.mebibytes(16),
      limit: Size.mebibytes(128),
    },
    ephemeralStorage: {
      limit: Size.gibibytes(1),
    },
  };
}

function zigbee2MqttProbe(failureThreshold: number): Probe {
  return Probe.fromHttpGet("/", {
    port: ZIGBEE2MQTT_HTTP_PORT,
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}

function initConfigMount(scope: Construct, configName: string): VolumeMount {
  const config = ConfigMap.fromConfigMapName(scope, "zigbee2mqtt-init-config", configName);
  return {
    volume: Volume.fromConfigMap(scope, "zigbee2mqtt-init-volume", config),
    path: INIT_MOUNT_PATH,
  };
}
