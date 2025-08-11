import { Construct } from "constructs";
import * as OnePassword from "@k2/1password";
import { ISecret, Secret } from "cdk8s-plus-28";

export class GlauthUsers extends OnePassword.crd.OnePasswordItem {
  constructor(scope: Construct, id: string) {
    super(scope, id, {
      spec: {
        itemPath: "vaults/zfsyjjcwge4w4gw6dh4zaqndhq/items/7p4cogd3voxt6sonqlj6jb3q4a",
      },
    });
  }

  public get secret(): ISecret {
    return Secret.fromSecretName(this, "secret", this.name);
  }
}
