import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Service } from "cdk8s-plus-32";

import { App, ApexDomain } from "@k2/cdk-lib";
import { K2Secret } from "@k2/1password";

import { GlauthDeployment } from "./deployment.js";
import { GlauthConfig } from "./config.js";

export class Glauth extends Chart {
  public readonly service: Service;

  constructor(scope: Construct, id: string) {
    super(scope, id);

    const { apexDomain } = ApexDomain.of(this);
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
