import { Cpu, Deployment, DeploymentStrategy, ImagePullPolicy, Probe } from "cdk8s-plus-32";
import { Construct } from "constructs";
import { Size } from "cdk8s";

import { oci } from "@k2/cdk-lib";

export class DmhyMcpDeployment extends Deployment {
  constructor(scope: Construct, id: string) {
    super(scope, id, {
      replicas: 1,
      strategy: DeploymentStrategy.recreate(),
      securityContext: {
        user: 65532,
        group: 65532,
      },
    });

    const probe = Probe.fromHttpGet("/healthz", { port: 8080 });

    this.addContainer({
      image: oci`ghcr.io/wyvernzora/dmhy-mcp:dev`,
      imagePullPolicy: ImagePullPolicy.ALWAYS,
      ports: [
        {
          name: "mcp",
          number: 8080,
        },
      ],
      securityContext: {
        ensureNonRoot: true,
        readOnlyRootFilesystem: true,
      },
      liveness: probe,
      readiness: probe,
      resources: {
        cpu: {
          request: Cpu.millis(25),
          limit: Cpu.millis(500),
        },
        memory: {
          request: Size.mebibytes(32),
          limit: Size.mebibytes(256),
        },
        ephemeralStorage: {
          limit: Size.gibibytes(1),
        },
      },
    });
  }
}
