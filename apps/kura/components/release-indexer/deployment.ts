import { Duration, Size } from "cdk8s";
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

import { K2Deployment, oci } from "@k2/cdk-lib";

import { KURA_RELEASE_INDEXER_HTTP_PORT, KURA_RELEASE_INDEXER_LABELS } from "../../constants.js";

import { RELEASE_INDEXER_CONFIG_KEY } from "./config.js";

const RELEASE_INDEXER_IMAGE = oci`ghcr.io/wyvernzora/kura/release-indexer:v0.6.1`;
const APP_UID = 65532;
const APP_GID = 65532;
const CONFIG_MOUNT_PATH = "/etc/kura/release-indexer.toml";

export interface ReleaseIndexerDeploymentProps {
  readonly configChecksum: string;
  readonly configName: string;
  readonly credentialsSecretName: string;
}

export class ReleaseIndexerDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: ReleaseIndexerDeploymentProps) {
    const config = ConfigMap.fromConfigMapName(scope, `${id}-config`, props.configName);
    const configVolume = Volume.fromConfigMap(scope, `${id}-config-volume`, config, { name: "config" });
    super(scope, id, {
      metadata: { name: "kura-release-indexer" },
      replicas: 0,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: {
        labels: KURA_RELEASE_INDEXER_LABELS,
        annotations: {
          "checksum/release-indexer-config": props.configChecksum,
        },
      },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
      },
      volumes: [configVolume],
    });

    this.select(LabelSelector.of({ labels: KURA_RELEASE_INDEXER_LABELS }));
    const credentials = Secret.fromSecretName(this, "credentials-secret", props.credentialsSecretName);
    this.addContainer(releaseIndexerContainer(configVolume, credentials));
  }
}

function releaseIndexerContainer(configVolume: Volume, credentials: ISecret): ContainerProps {
  const health = Probe.fromHttpGet("/healthz", {
    port: KURA_RELEASE_INDEXER_HTTP_PORT,
    failureThreshold: 6,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
  return {
    name: "release-indexer",
    image: RELEASE_INDEXER_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    args: [`--config=${CONFIG_MOUNT_PATH}`],
    ports: [{ name: "http", number: KURA_RELEASE_INDEXER_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      KURA_RELEASES_DATABASE_URL: credentials.envValue("uri"),
      TZ: EnvValue.fromValue("America/Los_Angeles"),
    },
    volumeMounts: [configMount(configVolume)],
    liveness: health,
    readiness: health,
    startup: Probe.fromHttpGet("/healthz", {
      port: KURA_RELEASE_INDEXER_HTTP_PORT,
      failureThreshold: 30,
      periodSeconds: Duration.seconds(10),
      timeoutSeconds: Duration.seconds(5),
    }),
    resources: {
      cpu: {
        request: Cpu.millis(50),
        limit: Cpu.millis(1000),
      },
      memory: {
        request: Size.mebibytes(128),
        limit: Size.gibibytes(1),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
    securityContext: {
      user: APP_UID,
      group: APP_GID,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
  };
}

function configMount(configVolume: Volume): VolumeMount {
  return {
    volume: configVolume,
    path: CONFIG_MOUNT_PATH,
    subPath: RELEASE_INDEXER_CONFIG_KEY,
    readOnly: true,
  };
}
