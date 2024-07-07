import { K2MaterializedVolume, K2Volumes, oci } from "@k2/cdk-lib";
import { Size } from "cdk8s";
import { ConfigMap, Cpu, Deployment, DeploymentStrategy, Volume, VolumeMount } from "cdk8s-plus-28";
import { Construct } from "constructs";
import dedent from "dedent-js";

const PUID = 3000;
const PGID = 2001;

export interface QBitTorrentDeploymentProps {
  readonly volumes: K2Volumes<"appdata">;
  readonly downloads: K2Volumes;
}
type Props = QBitTorrentDeploymentProps;

export class QBitTorrentDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    const appdata = props.volumes.appdata(this, `vol-appdata`);
    const downloads = this.createAdditionalVolumeMounts(props.downloads);
    this.addQbitTorrentContainer(appdata, downloads);
    this.addFloodUiContainer(appdata, downloads);
    this.initAppdataContainer(appdata);
  }

  // Create volume mounts for additional volumes that are for persisting downloaded files.
  private createAdditionalVolumeMounts(volumes: K2Volumes): VolumeMount[] {
    return Object.entries(volumes).map(([name, vol]) =>
      vol(this, `vol-${name}`).mount(this, { path: `/downloads/${name}` }),
    );
  }

  private initAppdataContainer(appdata: K2MaterializedVolume) {
    this.addInitContainer({
      name: "init-appdata",
      image: "busybox",
      command: ["sh", "/init/init.sh"],
      volumeMounts: [appdata.mount(this, { path: "/config" }), this.createInitScriptVolumeMount()],
      securityContext: {
        ensureNonRoot: false,
      },
    });
  }

  private addQbitTorrentContainer(appdata: K2MaterializedVolume, mounts: VolumeMount[]) {
    this.addContainer({
      name: "qbittorrent",
      image: oci`lscr.io/linuxserver/qbittorrent:20.04.1`,
      ports: [
        {
          name: "http",
          number: 8080,
        },
      ],
      volumeMounts: [
        ...mounts,
        appdata.mount(this, {
          path: "/config",
          subPath: "qbittorrent",
        }),
      ],
      envVariables: {
        PUID: { value: `${PUID}` },
        PGID: { value: `${PGID}` },
        TZ: { value: "America/Los_Angeles" },
        WEBUI_PORTS: { value: "8080/tcp" },
      },
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
      resources: {
        cpu: {
          request: Cpu.millis(100),
          limit: Cpu.millis(1000),
        },
        memory: {
          request: Size.gibibytes(0.5),
          limit: Size.gibibytes(4),
        },
        ephemeralStorage: {
          limit: Size.gibibytes(8),
        },
      },
    });
  }

  private addFloodUiContainer(appdata: K2MaterializedVolume, mounts: VolumeMount[]) {
    this.addContainer({
      name: "floodui",
      image: oci`jesec/flood:4.8.2`,
      ports: [
        {
          name: "http",
          number: 3000,
        },
      ],
      envVariables: {
        FLOOD_OPTION_AUTH: { value: "none" },
        FLOOD_OPTION_QBURL: { value: "http://127.0.0.1:8080" },
        FLOOD_OPTION_QBUSER: { value: "dummy" },
        FLOOD_OPTION_QBPASS: { value: "dummy" },
        FLOOD_OPTION_RUNDIR: { value: "/config" },
        FLOOD_OPTION_ALLOWEDPATH: { value: "/downloads" },
      },
      volumeMounts: [
        ...mounts,
        appdata.mount(this, {
          path: "/config",
          subPath: "flood",
        }),
      ],
      securityContext: {
        user: PUID,
        group: PGID,
        readOnlyRootFilesystem: false,
      },
    });
  }

  private createInitScriptVolumeMount(): VolumeMount {
    const qbittorrentConf = dedent`
      [Preferences]
      WebUI\\Address=127.0.0.1
      WebUI\\Port=8080
      WebUI\\HostHeaderValidation=false
      WebUI\\LocalHostAuth=false
    `;
    const initScript = dedent`
      #!/bin/sh
      set -exv

      # Create config directories
      mkdir -p /config/qbittorrent
      mkdir -p /config/flood

      # Initialize default qBittorrent configuration
      if [ ! -f "/config/qbittorrent/qBittorrent/qBittorrent.conf" ]; then
        echo "qBittorrent configuration not found; initializing a default one"
        mkdir -p /config/qbittorrent/qBittorrent
        cp /init/qbittorrent.conf /config/qbittorrent/qBittorrent/qBittorrent.conf
      fi

      # Set up permissions
      chown -R ${PUID}:${PGID} /config
    `;
    const cm = new ConfigMap(this, "cm-init", {
      data: {
        "qbittorrent.conf": qbittorrentConf,
        "init.sh": initScript,
      },
    });
    return {
      volume: Volume.fromConfigMap(this, "vol-init", cm),
      path: "/init",
    };
  }
}
