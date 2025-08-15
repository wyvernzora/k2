import { Construct } from "constructs";
import { AppOptionFunc } from "@k2/cdk-lib";

const VAULT_ID = "zfsyjjcwge4w4gw6dh4zaqndhq";

export class VaultContext {
  private static readonly ContextKey = "@k2/1password:vault";

  public static of(construct: Construct): VaultContext {
    return construct.node.getContext(VaultContext.ContextKey);
  }

  public static with(vaultId: string): AppOptionFunc {
    return app => {
      app.node.setContext(VaultContext.ContextKey, new VaultContext(vaultId));
    };
  }

  private constructor(public readonly vaultId: string) {}
}

export function withDefaultVault() {
  return VaultContext.with(VAULT_ID);
}
