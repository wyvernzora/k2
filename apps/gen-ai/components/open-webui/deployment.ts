import { Deployment, DeploymentStrategy, Probe } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { K2Volumes, VolumesOf, oci } from "@k2/cdk-lib";

export interface OpenWebUIDeploymentProps {
  readonly volumes: K2Volumes<"data">;
}
type Props = OpenWebUIDeploymentProps;

export class OpenWebUIDeployment extends Deployment {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
    });
    this.addOpenWebUIContainer(props.volumes);
  }

  private addOpenWebUIContainer(volumes: VolumesOf<Props>) {
    this.addContainer({
      name: "open-webui",
      image: oci`ghcr.io/open-webui/open-webui:0.6.43`,
      ports: [
        {
          name: "http",
          number: 8080,
        },
      ],
      volumeMounts: [
        volumes.data(this, "vol-data").mount(this, {
          path: "/app/backend/data",
        }),
      ],
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
      readiness: Probe.fromHttpGet("/", { port: 8080 }),
      liveness: Probe.fromHttpGet("/", { port: 8080 }),
    });
  }
}
