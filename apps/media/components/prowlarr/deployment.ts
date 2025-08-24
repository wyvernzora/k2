import { Size } from "cdk8s";
import { Cpu, Deployment, DeploymentStrategy, Probe } from "cdk8s-plus-28";
import { Construct } from "constructs";

import { K2MaterializedVolume, K2Volumes, oci } from "@k2/cdk-lib";

const PUID = 3000;
const PGID = 2001;

export interface ProwlarrDeploymentProps {
  readonly volumes: K2Volumes<"appdata">;
}
type Props = ProwlarrDeploymentProps;

export class ProwlarrDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    const configVolume = props.volumes.appdata(this, `vol-appdata`);
    this.addProwlarrContainer(configVolume);
  }

  private addProwlarrContainer(configVolume: K2MaterializedVolume) {
    this.addContainer({
      name: "prowlarr",
      image: oci`linuxserver/prowlarr:2.0.5`,
      ports: [
        {
          name: "http",
          number: 9696,
        },
      ],
      volumeMounts: [configVolume.mount(this, { path: "/config" })],
      envVariables: {
        PUID: { value: `${PUID}` },
        PGID: { value: `${PGID}` },
        TZ: { value: "America/Los_Angeles" },
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
      liveness: Probe.fromCommand(["wget", "-qO", "/dev/null", "http://127.0.0.1:9696"]),
      readiness: Probe.fromCommand(["wget", "-qO", "/dev/null", "http://127.0.0.1:9696"]),
    });
  }
}
