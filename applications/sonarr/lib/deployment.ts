import {
  Cpu,
  Deployment,
  DeploymentStrategy,
  VolumeMount,
} from "cdk8s-plus-28";
import { Construct } from "constructs";
import { K2Volume, K2Volumes } from "~lib";
import { Size } from "cdk8s";

export interface SonarrDeploymentProps {
  readonly volumes: K2Volumes<"anime" | "config">;
}
type Props = SonarrDeploymentProps;

export class SonarrDeployment extends Deployment {
  constructor(scope: Construct, id: string, { volumes }: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });

    const mounts = this.createVolumeMounts(volumes);
    this.addSonarrContainer(mounts);
  }

  private *createVolumeMounts(
    volumes: Props["volumes"],
  ): Iterable<VolumeMount> {
    yield volumes.config(this, "vol-config").mount(this, { path: "/config" });
    yield volumes.anime(this, "vol-anime").mount(this, { path: "/mnt/anime" });
    yield K2Volume.ephemeral()(this, "vol-backups").mount(this, {
      path: "/config/Backups",
    });
    yield K2Volume.ephemeral()(this, "vol-logs").mount(this, {
      path: "/config/logs",
    });
  }

  private addSonarrContainer(mounts: Iterable<VolumeMount>): void {
    this.addContainer({
      image: "quay.io/linuxserver.io/sonarr:4.0.1",
      ports: [
        {
          name: "http",
          number: 8989,
        },
      ],
      volumeMounts: [...mounts],
      envVariables: {
        PUID: { value: "3000" },
        PGID: { value: "2001" },
        TZ: { value: "America/Los_Angeles" },
      },
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
      resources: {
        cpu: {
          request: Cpu.millis(100),
          limit: Cpu.millis(2000),
        },
        memory: {
          request: Size.gibibytes(0.5),
          limit: Size.gibibytes(4),
        },
        ephemeralStorage: {
          limit: Size.gibibytes(10),
        },
      },
    });
  }
}
