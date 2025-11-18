import { Deployment, DeploymentStrategy } from "cdk8s-plus-32";
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
    });
    this.addAnythingLLMContainer(props.volumes);
  }

  private addAnythingLLMContainer(volumes: VolumesOf<Props>) {
    this.addContainer({
      name: "anything-llm",
      image: oci`mintplexlabs/anythingllm:v1.9.0`,
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
      securityContext: {
        ensureNonRoot: true,
        readOnlyRootFilesystem: true,
      },
    });
  }
}
