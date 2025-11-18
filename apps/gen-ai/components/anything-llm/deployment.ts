import { Deployment, DeploymentStrategy, EnvValue, Probe } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { K2Volumes, oci, VolumesOf } from "@k2/cdk-lib";

export interface AnythingLLMDeploymentProps {
  readonly volumes: K2Volumes<"appdata">;
}
type Props = AnythingLLMDeploymentProps;

export class AnythingLLMDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
      securityContext: {
        ensureNonRoot: true,
        user: 1000,
        group: 1000,
        fsGroup: 1000,
      },
    });
    this.addAnythingLLMContainer(props.volumes);
  }

  private addAnythingLLMContainer(volumes: VolumesOf<Props>) {
    this.addContainer({
      name: "anything-llm",
      image: oci`mintplexlabs/anythingllm:1.9.0`,
      ports: [
        {
          name: "http",
          number: 3001,
        },
      ],
      volumeMounts: [
        volumes.appdata(this, "vol-appdata").mount(this, {
          path: "/app/server/storage",
        }),
      ],
      envVariables: {
        STORAGE_DIR: EnvValue.fromValue("/app/server/storage"),
      },
      securityContext: {
        readOnlyRootFilesystem: false,
      },
      readiness: Probe.fromHttpGet("/", { port: 3001 }),
      liveness: Probe.fromHttpGet("/", { port: 3001 }),
    });
  }
}
