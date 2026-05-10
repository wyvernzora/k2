import { Cpu, Deployment, DeploymentStrategy, EnvValue, ImagePullPolicy, Probe, VolumeMount } from "cdk8s-plus-32";
import { Construct } from "constructs";
import { Size } from "cdk8s";

import { K2Volumes, oci } from "@k2/cdk-lib";
import { K2Secret } from "@k2/1password";

export interface KuraDeploymentProps {
  readonly volumes: K2Volumes<"anime">;
}
type Props = KuraDeploymentProps;

export class KuraDeployment extends Deployment {
  constructor(scope: Construct, id: string, { volumes }: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
      securityContext: {
        user: 3000,
        group: 2001,
        fsGroup: 2001,
      },
    });

    const mounts = [...this.createVolumeMounts(volumes)];
    this.addKuraContainer(mounts);
  }

  private *createVolumeMounts(volumes: Props["volumes"]): Iterable<VolumeMount> {
    yield volumes.anime(this, "vol-anime").mount(this, { path: "/anime" });
  }

  private addKuraContainer(mounts: VolumeMount[]): void {
    const probe = Probe.fromHttpGet("/api/v1/health", { port: 8080 });

    const tvdb = new K2Secret(this, "tvdb", {
      itemId: "q4xf32di7npmvc7e62amvgd574",
    });

    this.addContainer({
      image: oci`ghcr.io/wyvernzora/kura:dev`,
      imagePullPolicy: ImagePullPolicy.ALWAYS,
      ports: [
        {
          name: "http",
          number: 8080,
        },
        {
          name: "mcp",
          number: 8081,
        },
      ],
      volumeMounts: mounts,
      envVariables: {
        KURA_LIBRARY_ROOT: { value: "/anime/series" },
        KURA_INBOX_ROOT: { value: "/anime/downloads" },
        KURA_DISABLE_TOKEN: { value: "1" },
        KURA_HOST_ID: { value: "k2-media-kura" },
        KURA_PREFERRED_LANGUAGES: { value: "ja" },
        KURA_TVDB_KEY: EnvValue.fromSecretValue({
          secret: tvdb.secret,
          key: "credential",
        }),
        TZ: { value: "America/Los_Angeles" },
      },
      securityContext: {
        ensureNonRoot: true,
        readOnlyRootFilesystem: true,
      },
      liveness: probe,
      readiness: probe,
      resources: {
        cpu: {
          request: Cpu.millis(100),
          limit: Cpu.millis(2000),
        },
        memory: {
          request: Size.gibibytes(0.25),
          limit: Size.gibibytes(2),
        },
        ephemeralStorage: {
          limit: Size.gibibytes(2),
        },
      },
    });
  }
}
