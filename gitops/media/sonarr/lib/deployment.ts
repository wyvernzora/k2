import {
  Cpu,
  Deployment,
  DeploymentStrategy,
  VolumeMount,
} from "cdk8s-plus-27";
import { Construct } from "constructs";
import { VolumeProps, createVolume } from "~lib";
import { Size } from "cdk8s";

export interface SonarrDeploymentProps {
  readonly volumes: SonarrVolumes;
}
export interface SonarrVolumes {
  readonly anime: VolumeProps;
  readonly config: VolumeProps;
}

export class SonarrDeployment extends Deployment {
  constructor(
    scope: Construct,
    id: string,
    { volumes }: SonarrDeploymentProps,
  ) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });

    const mounts = this.createVolumeMounts(volumes);
    this.addSonarrContainer(mounts);
  }

  private createVolumeMounts(volumes: SonarrVolumes): VolumeMount[] {
    const ephemeral = createVolume(this, "vol-eph", { kind: "ephemeral" });
    return [
      createVolume(this, `vol-config`, volumes.config).mount({
        path: "/config",
      }),
      createVolume(this, `vol-anime`, volumes.anime).mount({
        path: "/mnt/anime",
      }),
      // Make backups and logs ephemeral since we do not use this
      ephemeral.mount({ path: "/config/Backups" }),
      ephemeral.mount({ path: "/config/logs" }),
    ];
  }

  private addSonarrContainer(mounts: VolumeMount[]): void {
    this.addContainer({
      image: "quay.io/linuxserver.io/sonarr:4.0.1",
      ports: [
        {
          name: "http",
          number: 8989,
        },
      ],
      volumeMounts: mounts,
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
