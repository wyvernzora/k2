import { k8s } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { Scheduling } from "@k2/cdk-lib";

const BLOCKY_IMAGE = "ghcr.io/0xerr0r/blocky:v0.26.2";
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

export class BlockyDaemonSet extends k8s.KubeDaemonSet {
  public readonly selectorLabels = { ...POD_LABELS };

  public constructor(scope: Construct, id: string, props: BlockyDaemonSetProps) {
    super(scope, id, blockyDaemonSetProps(props));
  }
}

function blockyDaemonSetProps(props: BlockyDaemonSetProps): k8s.KubeDaemonSetProps {
  return {
    metadata: {
      name: "blocky",
    },
    spec: {
      selector: blockySelector(),
      template: blockyPodTemplate(props),
    },
  };
}

function blockySelector(): k8s.LabelSelector {
  return {
    matchLabels: { ...POD_LABELS },
  };
}

function blockyPodTemplate(props: BlockyDaemonSetProps): k8s.PodTemplateSpec {
  return {
    metadata: {
      labels: { ...POD_LABELS },
      annotations: {
        "checksum/blocky-config": props.configChecksum,
      },
    },
    spec: blockyPodSpec(props.configName),
  };
}

function blockyPodSpec(configName: string): k8s.PodSpec {
  const scheduling = Scheduling.workersPreferred();
  return {
    automountServiceAccountToken: false,
    enableServiceLinks: false,
    affinity: scheduling.affinity,
    tolerations: scheduling.tolerations,
    containers: [blockyContainer()],
    volumes: [
      {
        name: CONFIG_VOLUME_NAME,
        configMap: {
          name: configName,
        },
      },
    ],
  };
}

function blockyContainer(): k8s.Container {
  return {
    name: "blocky",
    image: BLOCKY_IMAGE,
    ports: [
      { name: "dns-udp", containerPort: 53, protocol: "UDP" },
      { name: "dns-tcp", containerPort: 53, protocol: "TCP" },
      { name: "http", containerPort: 4000, protocol: "TCP" },
    ],
    env: [{ name: "TZ", value: "America/Los_Angeles" }],
    volumeMounts: [{ name: CONFIG_VOLUME_NAME, mountPath: CONFIG_MOUNT_PATH, subPath: CONFIG_KEY }],
    resources: {
      requests: resourceList("100m", "256Mi"),
      limits: resourceList("250m", "1024Mi"),
    },
  };
}

function resourceList(cpu: string, memory: string): Record<string, k8s.Quantity> {
  return {
    cpu: k8s.Quantity.fromString(cpu),
    memory: k8s.Quantity.fromString(memory),
  };
}
