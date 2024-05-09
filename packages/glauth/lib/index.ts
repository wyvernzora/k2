import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { GlauthConfig } from "./config";
import { GlauthDeployment } from "./deployment";
import { Service } from "cdk8s-plus-28";
import { GlauthUsers } from "./users";

export class GlauthChart extends Chart {
  public readonly service: Service;

  constructor(scope: Construct, id: string) {
    super(scope, id);

    const config = new GlauthConfig(this, "config", {
      domain: "wyvernzora.io",
      ldapPort: 389,
    });
    const users = new GlauthUsers(this, "users");
    const deployment = new GlauthDeployment(this, "depl", {
      config: config,
      users: users,
    });
    this.service = deployment.exposeViaService({ name: "glauth" });
  }
}
