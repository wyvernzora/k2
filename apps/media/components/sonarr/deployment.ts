import { Cpu, Deployment, DeploymentStrategy, VolumeMount } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { Size } from "cdk8s";

import { K2Volume, K2Volumes, oci } from "@k2/cdk-lib";

export interface SonarrDeploymentProps {
  readonly volumes: K2Volumes<"anime" | "appdata">;
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

  private *createVolumeMounts(volumes: Props["volumes"]): Iterable<VolumeMount> {
    yield volumes.appdata(this, "vol-appdata").mount(this, { path: "/config" });
    yield volumes.anime(this, "vol-anime").mount(this, { path: "/mnt/anime" });

    // Do not persist backups; these are handled at volume level by Kubernetes
    yield K2Volume.ephemeral()(this, "vol-backups").mount(this, {
      path: "/config/Backups",
    });

    // Do not persist logs beyond pod lifetime.
    yield K2Volume.ephemeral()(this, "vol-logs").mount(this, {
      path: "/config/logs",
    });
  }

  private addSonarrContainer(mounts: Iterable<VolumeMount>): void {
    this.addContainer({
      image: oci`quay.io/linuxserver.io/sonarr:4.0.15`,
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
