import { K2Volumes } from "@k2/cdk-lib";
import { Construct } from "constructs";
import {
  Cpu,
  Deployment,
  DeploymentStrategy,
  VolumeMount,
} from "cdk8s-plus-28";
import { Size } from "cdk8s";

export interface QBitTorrentDeploymentProps {
  readonly volumes: K2Volumes<"config" | "anime" | "airing" | "default">;
}
type Props = QBitTorrentDeploymentProps;

export class QBitTorrentDeployment extends Deployment {
  constructor(scope: Construct, id: string, { volumes }: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    const mounts = this.createVolumeMounts(volumes);
    this.addQbitTorrentContainer(mounts);
  }

  private *createVolumeMounts(
    volumes: Props["volumes"],
  ): IterableIterator<VolumeMount> {
    yield volumes.config(this, "vol-config").mount(this, { path: "/config" });
    yield volumes
      .default(this, "vol-default")
      .mount(this, { path: "/downloads/default" });
    yield volumes
      .anime(this, "vol-anime")
      .mount(this, { path: "/downloads/anime" });
    yield volumes
      .airing(this, "vol-airing")
      .mount(this, { path: "/downloads/airing" });
  }

  private addQbitTorrentContainer(mounts: Iterable<VolumeMount>): void {
    this.addContainer({
      image: "ghcr.io/hotio/qbittorrent:release-4.6.3",
      ports: [
        {
          name: "http",
          number: 8080,
        },
      ],
      volumeMounts: [...mounts],
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
