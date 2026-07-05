import { ApiObject, Duration, JsonPatch, Size } from "cdk8s";
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
  Secret,
  Volume,
  type ContainerProps,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

import { K2Deployment, oci, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { PLEX_CADDY_HTTP_REDIRECT_PORT, PLEX_CADDY_PORT, PLEX_HTTP_PORT, PLEX_LABELS } from "../../constants.js";

const PLEX_IMAGE = oci`plexinc/pms-docker:1.43.2.10687-563d026ea-armhf`;
const CADDY_IMAGE = oci`caddy:2.11.4-alpine`;
const BUSYBOX_IMAGE = oci`busybox:1.38.0`;
const DEFAULT_CERTIFICATE_SECRET_NAME = "default-certificate";
const PUID = 3001;
const PGID = 2001;
const MEDIA_ACCESS_GID = 2001;
const CONFIG_MOUNT_PATH = "/config";
const PLEX_ROOT = `${CONFIG_MOUNT_PATH}/Library/Application Support/Plex Media Server`;
const DATABASES_PATH = `${PLEX_ROOT}/Plug-in Support/Databases`;
const DATABASES_INIT_PATH = "/databases";
const TRANSCODE_PATH = "/transcode";
const CONFIG_READY_MARKER = `${CONFIG_MOUNT_PATH}/.k2-plex-config-initialized`;
const DATABASES_PERMISSIONS_MARKER = `${DATABASES_INIT_PATH}/.k2-plex-permissions-${PUID}-${PGID}-initialized`;

export interface PlexDeploymentProps {
  readonly volumes: K2Volumes;
}

export class PlexDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: PlexDeploymentProps) {
    super(scope, id, {
      metadata: { name: "plex" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: PLEX_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: false,
      },
    });

    this.select(LabelSelector.of({ labels: PLEX_LABELS }));
    ApiObject.of(this).addJsonPatch(
      JsonPatch.remove("/spec/template/spec/securityContext/fsGroupChangePolicy"),
      JsonPatch.add("/spec/template/spec/securityContext/supplementalGroups", [MEDIA_ACCESS_GID]),
    );
    const volumes = this.attachVolumes(props.volumes);
    const caddyMounts = caddyVolumeMounts(this);
    this.addInitContainer(initConfigContainer(volumes));
    this.addInitContainer(initDatabaseContainer(volumes));
    this.addContainer(plexContainer(volumes));
    this.addContainer(caddyContainer(caddyMounts));
  }
}

function initConfigContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "init-plex-config",
    image: BUSYBOX_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: ["sh", "-c", initConfigScript()],
    volumeMounts: [volumes.config(CONFIG_MOUNT_PATH)],
    resources: initResources(),
    securityContext: {
      user: PUID,
      group: PGID,
      ensureNonRoot: true,
    },
  };
}

function initDatabaseContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "init-plex-databases",
    image: BUSYBOX_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: ["sh", "-c", initDatabaseScript()],
    volumeMounts: [volumes.databases(DATABASES_INIT_PATH)],
    resources: initResources(),
    securityContext: {
      user: 0,
      group: 0,
      ensureNonRoot: false,
    },
  };
}

function initResources(): ContainerProps["resources"] {
  return {
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(500),
      },
      memory: {
        request: Size.mebibytes(32),
        limit: Size.mebibytes(256),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(1),
      },
    },
  }.resources;
}

function plexContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "plex",
    image: PLEX_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: PLEX_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      PLEX_UID: EnvValue.fromValue(String(PUID)),
      PLEX_GID: EnvValue.fromValue(String(PGID)),
      TZ: EnvValue.fromValue("America/Los_Angeles"),
      VERSION: EnvValue.fromValue("docker"),
      ADVERTISE_IP: EnvValue.fromValue("https://plex.wyvernzora.io"),
      CHANGE_CONFIG_DIR_OWNERSHIP: EnvValue.fromValue("false"),
    },
    volumeMounts: [
      volumes.config(CONFIG_MOUNT_PATH),
      volumes.databases(DATABASES_PATH),
      volumes.series("/anime/series"),
      volumes.features("/anime/features"),
      volumes.transcode(TRANSCODE_PATH),
    ],
    liveness: plexHttpProbe(6),
    readiness: plexHttpProbe(3),
    startup: plexHttpProbe(60),
    resources: {
      cpu: {
        request: Cpu.millis(250),
        limit: Cpu.millis(4000),
      },
      memory: {
        request: Size.gibibytes(1),
        limit: Size.gibibytes(8),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(8),
      },
    },
    securityContext: {
      ensureNonRoot: false,
      readOnlyRootFilesystem: false,
    },
  };
}

function caddyContainer(volumeMounts: VolumeMount[]): ContainerProps {
  return {
    name: "caddy",
    image: CADDY_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [
      { name: "http", number: PLEX_CADDY_HTTP_REDIRECT_PORT, protocol: Protocol.TCP },
      { name: "https", number: PLEX_CADDY_PORT, protocol: Protocol.TCP },
    ],
    volumeMounts,
    envVariables: {
      XDG_CONFIG_HOME: EnvValue.fromValue("/config"),
      XDG_DATA_HOME: EnvValue.fromValue("/data"),
    },
    liveness: caddyTcpProbe(6),
    readiness: caddyTcpProbe(3),
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
      user: 65532,
      group: 65532,
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

function plexHttpProbe(failureThreshold: number): Probe {
  return Probe.fromHttpGet("/identity", {
    port: PLEX_HTTP_PORT,
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}

function caddyTcpProbe(failureThreshold: number): Probe {
  return Probe.fromTcpSocket({
    port: PLEX_CADDY_PORT,
    failureThreshold,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}

function caddyVolumeMounts(scope: Construct): VolumeMount[] {
  const configMap = new ConfigMap(scope, "caddy-config", {
    data: {
      Caddyfile: caddyfile(),
    },
  });
  const certificate = Secret.fromSecretName(scope, "default-certificate", DEFAULT_CERTIFICATE_SECRET_NAME);
  return [
    {
      volume: Volume.fromConfigMap(scope, "caddy-config-volume", configMap),
      path: "/etc/caddy/Caddyfile",
      subPath: "Caddyfile",
    },
    {
      volume: Volume.fromSecret(scope, "default-certificate-volume", certificate),
      path: "/tls",
      readOnly: true,
    },
    {
      volume: Volume.fromEmptyDir(scope, "caddy-config-state-volume", "caddy-config-state", {
        sizeLimit: Size.gibibytes(1),
      }),
      path: "/config",
    },
    {
      volume: Volume.fromEmptyDir(scope, "caddy-data-volume", "caddy-data", {
        sizeLimit: Size.gibibytes(1),
      }),
      path: "/data",
    },
    {
      volume: Volume.fromEmptyDir(scope, "caddy-tmp-volume", "caddy-tmp", {
        sizeLimit: Size.gibibytes(1),
      }),
      path: "/tmp",
    },
  ];
}

function initConfigScript(): string {
  return dedent`
    set -eu

    mkdir -p "${PLEX_ROOT}/Plug-in Support"
    mkdir -p "${PLEX_ROOT}/Plug-in Support/Databases"
    touch "${CONFIG_READY_MARKER}"
  `;
}

function initDatabaseScript(): string {
  return dedent`
    set -eu

    mkdir -p "${DATABASES_INIT_PATH}"
    if [ ! -f "${DATABASES_PERMISSIONS_MARKER}" ]; then
      chown -R ${PUID}:${PGID} "${DATABASES_INIT_PATH}"
      touch "${DATABASES_PERMISSIONS_MARKER}"
      chown ${PUID}:${PGID} "${DATABASES_PERMISSIONS_MARKER}"
    fi
  `;
}

function caddyfile(): string {
  return dedent`
    {
      auto_https off
    }

    :${PLEX_CADDY_HTTP_REDIRECT_PORT} {
      redir https://{host}{uri} permanent
    }

    :${PLEX_CADDY_PORT} {
      tls /tls/tls.crt /tls/tls.key
      reverse_proxy 127.0.0.1:${PLEX_HTTP_PORT}
    }
  `;
}
