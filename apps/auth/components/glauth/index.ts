import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { GlauthConfig } from "./config";
import { GlauthDeployment } from "./deployment";
import { Service } from "cdk8s-plus-28";
import { App, ApexDomainContext } from "@k2/cdk-lib";
import { K2Secret } from "@k2/1password";

export class Glauth extends Chart {
  public readonly service: Service;

  constructor(scope: Construct, id: string) {
    super(scope, id);

    const { apexDomain } = ApexDomainContext.of(this);
    const config = new GlauthConfig(this, "config", {
      domain: apexDomain,
      ldapPort: 389,
    });
    const users = new K2Secret(this, "users", {
      itemId: "7p4cogd3voxt6sonqlj6jb3q4a",
    });
    const deployment = new GlauthDeployment(this, "depl", {
      config: config,
      users: users.secret,
    });
    this.service = deployment.exposeViaService({ name: "glauth" });
  }
}

export default {
  create(app: App) {
    new Glauth(app, "glauth");
  },
};
