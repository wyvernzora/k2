import { ApiObjectMetadata } from "cdk8s";
import { ISecret, Secret } from "cdk8s-plus-28";
import { Construct } from "constructs";

import { OnePasswordItem } from "../crds/onepassword.com.js";

import { VaultContext } from "./context.js";

export interface K2SecretProps {
  readonly metadata?: ApiObjectMetadata;
  readonly itemId: string;
}

/**
 * K2 cluster uses 1Password to inject secrets.
 * This is a more concise wrapper since all K2 secrets live in a
 * dedicated 1Password vault for isolation.
 */
export class K2Secret extends OnePasswordItem {
  public readonly secret: ISecret;

  constructor(scope: Construct, id: string, props: K2SecretProps) {
    const ctx = VaultContext.of(scope);
    super(scope, id, {
      metadata: props.metadata,
      spec: {
        itemPath: `vaults/${ctx.vaultId}/items/${props.itemId}`,
      },
    });
    this.secret = Secret.fromSecretName(this, "secret", this.name);
  }
}
