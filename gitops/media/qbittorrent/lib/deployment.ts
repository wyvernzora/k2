import { createVolume, VolumeProps } from "~lib";
import { Construct } from "constructs";
import {
  Cpu,
  Deployment,
  DeploymentStrategy,
  VolumeMount,
} from "cdk8s-plus-27";
import { Size } from "cdk8s";

export interface QbitTorrentDeploymentProps {
  readonly volumes: QbitTorrentVolumes;
}

export interface QbitTorrentVolumes {
  readonly config: VolumeProps;
  readonly downloads: Record<string, VolumeProps>;
}

export class QbitTorrentDeployment extends Deployment {
  constructor(
    scope: Construct,
    id: string,
    { volumes }: QbitTorrentDeploymentProps,
  ) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });

    const mounts = this.createVolumeMounts(volumes);
    this.addQbitTorrentContainer(mounts);
  }

  private createVolumeMounts(volumes: QbitTorrentVolumes): VolumeMount[] {
    const mounts = [
      createVolume(this, "vol-config", volumes.config).mount({
        path: "/config",
      }),
    ];
    Object.entries(volumes.downloads)
      .map(([name, props]) =>
        createVolume(this, `vol-${name}`, props).mount({
          path: `/data/${name}`,
        }),
      )
      .forEach((vol) => mounts.push(vol));
    return mounts;
  }

  private addQbitTorrentContainer(mounts: VolumeMount[]): void {
    this.addContainer({
      image: "ghcr.io/hotio/qbittorrent:release-4.6.3",
      ports: [
        {
          name: "http",
          number: 8080,
        },
      ],
      volumeMounts: mounts,
      envVariables: {
        PUID: { value: "3000" },
        PGID: { value: "2001" },
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
}
