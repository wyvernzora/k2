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

import { K2Deployment, oci, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { HOME_ASSISTANT_HTTP_PORT, HOME_ASSISTANT_LABELS } from "../../constants.js";

const HOME_ASSISTANT_IMAGE = oci`linuxserver/homeassistant:2026.6.3`;
const CONFIG_MOUNT_PATH = "/config";
const INIT_MOUNT_PATH = "/init";

export interface HomeAssistantDeploymentProps {
  readonly configChecksum: string;
  readonly configName: string;
  readonly volumes: K2Volumes;
}

export class HomeAssistantDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: HomeAssistantDeploymentProps) {
    super(scope, id, {
      metadata: { name: "home-assistant" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: {
        labels: HOME_ASSISTANT_LABELS,
        annotations: {
          "checksum/home-assistant-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: false,
      },
    });

    this.select(LabelSelector.of({ labels: HOME_ASSISTANT_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    const initMount = initConfigMount(this, props.configName);
    this.addInitContainer(initConfigContainer(volumes, initMount));
    this.addContainer(homeAssistantContainer(volumes));
  }
}

function initConfigContainer(volumes: K2Mounters<K2Volumes>, initMount: VolumeMount): ContainerProps {
  return {
    name: "setup-config",
    image: oci`busybox:1.38.0`,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: ["sh", `${INIT_MOUNT_PATH}/init.sh`],
    volumeMounts: [volumes.config(CONFIG_MOUNT_PATH), initMount],
    resources: initResources(),
    securityContext: {
      ensureNonRoot: false,
      readOnlyRootFilesystem: true,
    },
  };
}

function homeAssistantContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  const health = homeAssistantProbe(3);
  return {
    name: "home-assistant",
    image: HOME_ASSISTANT_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: HOME_ASSISTANT_HTTP_PORT, protocol: Protocol.TCP }],
    volumeMounts: [volumes.config(CONFIG_MOUNT_PATH)],
    liveness: health,
    readiness: health,
    startup: homeAssistantProbe(60),
    resources: {
      cpu: {
        request: Cpu.millis(100),
        limit: Cpu.millis(2000),
      },
      memory: {
        request: Size.mebibytes(512),
        limit: Size.gibibytes(4),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(4),
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

function homeAssistantProbe(failureThreshold: number): Probe {
  return Probe.fromHttpGet("/", {
    port: HOME_ASSISTANT_HTTP_PORT,
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}

function initConfigMount(scope: Construct, configName: string): VolumeMount {
  const config = ConfigMap.fromConfigMapName(scope, "home-assistant-init-config", configName);
  return {
    volume: Volume.fromConfigMap(scope, "home-assistant-init-volume", config),
    path: INIT_MOUNT_PATH,
  };
}
