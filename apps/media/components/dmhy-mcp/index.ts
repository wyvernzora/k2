import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Deployment, Service } from "cdk8s-plus-32";

import { DmhyMcpDeployment } from "./deployment.js";

export class DmhyMcp extends Chart {
  readonly deployment: Deployment;
  readonly service: Service;

  constructor(scope: Construct, id: string) {
    super(scope, id, {});

    this.deployment = new DmhyMcpDeployment(this, "depl");
    this.service = this.deployment.exposeViaService({
      name: "dmhy-mcp",
      ports: [
        {
          port: 80,
          targetPort: 8080,
        },
      ],
    });
  }
}
