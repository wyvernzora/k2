import { Cpu, Deployment, DeploymentStrategy, Probe, VolumeMount } from "cdk8s-plus-28";
import { Construct } from "constructs";
import { Size } from "cdk8s";

import { K2Volumes, oci, VolumesOf } from "@k2/cdk-lib";

export interface KavitaDeploymentProps {
  readonly volumes: K2Volumes<"appdata" | "library">;
}
type Props = KavitaDeploymentProps;

export class KavitaDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    this.addKavitaContainer(props);
  }

  private addKavitaContainer(props: Props) {
    this.addContainer({
      name: "kavita",
      image: oci`linuxserver/kavita:0.8.7`,
      ports: [
        {
          name: "http",
          number: 5000,
        },
      ],
      volumeMounts: [...this.createVolumeMounts(props.volumes)],
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
      liveness: Probe.fromHttpGet("/", { port: 5000 }),
      readiness: Probe.fromHttpGet("/", { port: 5000 }),
    });
  }

  private *createVolumeMounts(volumes: VolumesOf<Props>): Iterable<VolumeMount> {
    yield volumes.appdata(this, "vol-appdata").mount(this, { path: "/config" });
    yield volumes.library(this, "vol-library").mount(this, { path: "/library" });
  }
}
