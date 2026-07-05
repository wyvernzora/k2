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
  Volume,
  type ContainerProps,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

import { K2Deployment, oci, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";

import { FLOOD_HTTP_PORT, QBITTORRENT_HTTP_PORT, QBITTORRENT_LABELS, QBIT_BRIDGE_PORT } from "../../constants.js";

const QBITTORRENT_IMAGE = oci`lscr.io/linuxserver/qbittorrent:4.6.7`;
const FLOOD_IMAGE = oci`jesec/flood:4.14.3`;
const QBIT_BRIDGE_IMAGE = oci`ghcr.io/wyvernzora/qbit-bridge:dev`;
const PUID = 3005;
const PGID = 2001;
const UMASK = "0007";
const APPDATA_MOUNT_PATH = "/config";
const DEFAULT_SAVE_PATH = "/downloads/anime/downloads";
const OTHER_SAVE_PATH = "/downloads/default";
const QBITTORRENT_SAVE_PATHS = `kura-inbox=${DEFAULT_SAVE_PATH},other=${OTHER_SAVE_PATH}`;

export interface QbittorrentDeploymentProps {
  readonly volumes: K2Volumes;
}

export class QbittorrentDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: QbittorrentDeploymentProps) {
    super(scope, id, {
      metadata: { name: "qbittorrent" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: QBITTORRENT_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: false,
      },
    });

    this.select(LabelSelector.of({ labels: QBITTORRENT_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    const initMount = initScriptMount(this);
    this.addInitContainer(initAppdataContainer(volumes, initMount));
    this.addInitContainer(initDownloadsContainer(volumes, initMount));
    this.addContainer(qbittorrentContainer(volumes));
    this.addContainer(floodContainer(volumes));
    this.addContainer(qbitBridgeContainer());
  }
}

function initAppdataContainer(volumes: K2Mounters<K2Volumes>, initMount: VolumeMount): ContainerProps {
  return {
    name: "init-appdata",
    image: oci`busybox:1.38.0`,
    command: ["sh", "/init/appdata.sh"],
    volumeMounts: [volumes.appdata(APPDATA_MOUNT_PATH), initMount],
    securityContext: {
      ensureNonRoot: false,
    },
  };
}

function initDownloadsContainer(volumes: K2Mounters<K2Volumes>, initMount: VolumeMount): ContainerProps {
  return {
    name: "init-downloads",
    image: oci`busybox:1.38.0`,
    command: ["sh", "/init/downloads.sh"],
    volumeMounts: [volumes.anime("/downloads/anime"), volumes.default("/downloads/default"), initMount],
    securityContext: {
      user: PUID,
      group: PGID,
      ensureNonRoot: true,
    },
  };
}

function qbittorrentContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "qbittorrent",
    image: QBITTORRENT_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: QBITTORRENT_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      PUID: EnvValue.fromValue(String(PUID)),
      PGID: EnvValue.fromValue(String(PGID)),
      UMASK: EnvValue.fromValue(UMASK),
      TZ: EnvValue.fromValue("America/Los_Angeles"),
      WEBUI_PORTS: EnvValue.fromValue(`${QBITTORRENT_HTTP_PORT}/tcp`),
    },
    volumeMounts: [
      volumes.default("/downloads/default"),
      volumes.anime("/downloads/anime"),
      volumes.appdata(APPDATA_MOUNT_PATH, { subPath: "qbittorrent" }),
    ],
    liveness: Probe.fromCommand(["wget", "-qO", "/dev/null", `http://127.0.0.1:${QBITTORRENT_HTTP_PORT}`]),
    readiness: Probe.fromCommand(["wget", "-qO", "/dev/null", `http://127.0.0.1:${QBITTORRENT_HTTP_PORT}`]),
    resources: {
      cpu: {
        request: Cpu.millis(100),
        limit: Cpu.millis(1000),
      },
      memory: {
        request: Size.mebibytes(512),
        limit: Size.gibibytes(4),
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

function floodContainer(volumes: K2Mounters<K2Volumes>): ContainerProps {
  return {
    name: "floodui",
    image: FLOOD_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: FLOOD_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      FLOOD_OPTION_AUTH: EnvValue.fromValue("none"),
      FLOOD_OPTION_QBURL: EnvValue.fromValue(`http://127.0.0.1:${QBITTORRENT_HTTP_PORT}`),
      FLOOD_OPTION_QBUSER: EnvValue.fromValue("dummy"),
      FLOOD_OPTION_QBPASS: EnvValue.fromValue("dummy"),
      FLOOD_OPTION_RUNDIR: EnvValue.fromValue(APPDATA_MOUNT_PATH),
      FLOOD_OPTION_ALLOWEDPATH: EnvValue.fromValue("/downloads"),
    },
    volumeMounts: [
      volumes.default("/downloads/default", { readOnly: true }),
      volumes.anime("/downloads/anime", { readOnly: true }),
      volumes.appdata(APPDATA_MOUNT_PATH, { subPath: "flood" }),
    ],
    liveness: Probe.fromHttpGet("/", { port: FLOOD_HTTP_PORT }),
    readiness: Probe.fromHttpGet("/", { port: FLOOD_HTTP_PORT }),
    securityContext: {
      user: PUID,
      group: PGID,
      readOnlyRootFilesystem: false,
    },
  };
}

function qbitBridgeContainer(): ContainerProps {
  const probe = Probe.fromHttpGet("/healthz", { port: QBIT_BRIDGE_PORT });
  return {
    name: "qbit-bridge",
    image: QBIT_BRIDGE_IMAGE,
    imagePullPolicy: ImagePullPolicy.ALWAYS,
    args: ["--transport=http", `--addr=:${QBIT_BRIDGE_PORT}`],
    ports: [{ name: "http", number: QBIT_BRIDGE_PORT, protocol: Protocol.TCP }],
    envVariables: {
      QBITTORRENT_URL: EnvValue.fromValue(`http://127.0.0.1:${QBITTORRENT_HTTP_PORT}`),
      QBITTORRENT_SAVE_PATHS: EnvValue.fromValue(QBITTORRENT_SAVE_PATHS),
    },
    liveness: probe,
    readiness: probe,
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
    securityContext: {
      user: 65532,
      group: 65532,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
  };
}

function initScriptMount(scope: Construct): VolumeMount {
  const configMap = new ConfigMap(scope, "qbittorrent-init", {
    data: {
      "appdata.sh": appdataInitScript(),
      "downloads.sh": downloadsInitScript(),
      "qbittorrent.conf": qbittorrentConfig(),
    },
  });
  return {
    volume: Volume.fromConfigMap(scope, "qbittorrent-init-volume", configMap),
    path: "/init",
  };
}

function qbittorrentConfig(): string {
  return dedent`
    [BitTorrent]
    Session\\AddExtensionToIncompleteFiles=true
    Session\\DefaultSavePath=${DEFAULT_SAVE_PATH}
    Session\\TempPathEnabled=false

    [Preferences]
    Downloads\\SavePath=${DEFAULT_SAVE_PATH}
    Downloads\\TempPathEnabled=false
    Downloads\\UseIncompleteExtension=true
    WebUI\\Address=0.0.0.0
    WebUI\\HostHeaderValidation=false
    WebUI\\LocalHostAuth=false
    WebUI\\Port=${QBITTORRENT_HTTP_PORT}
  `;
}

function appdataInitScript(): string {
  return dedent`
    #!/bin/sh
    set -eu
    umask ${UMASK}

    mkdir -p /config/qbittorrent/qBittorrent
    mkdir -p /config/flood

    if [ ! -f "/config/qbittorrent/qBittorrent/qBittorrent.conf" ]; then
      cp /init/qbittorrent.conf /config/qbittorrent/qBittorrent/qBittorrent.conf
    fi

    chown -R ${PUID}:${PGID} /config
  `;
}

function downloadsInitScript(): string {
  return dedent`
    #!/bin/sh
    set -eu
    umask ${UMASK}

    mkdir -p ${DEFAULT_SAVE_PATH}
    mkdir -p ${OTHER_SAVE_PATH}
  `;
}
