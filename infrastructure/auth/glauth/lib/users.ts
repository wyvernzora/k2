import { Construct } from "constructs";
import { OnePasswordItem } from "~crds/onepassword.com";
import { ISecret, Secret } from "cdk8s-plus-27";

export class GlauthUsers extends OnePasswordItem {
  constructor(scope: Construct, id: string) {
    super(scope, id, {
      spec: {
        itemPath:
          "vaults/zfsyjjcwge4w4gw6dh4zaqndhq/items/7p4cogd3voxt6sonqlj6jb3q4a",
      },
    });
  }

  public get secret(): ISecret {
    return Secret.fromSecretName(this, "secret", this.name);
  }
}
