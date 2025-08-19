import { Context } from "@k2/cdk-lib";

const VAULT_ID = "zfsyjjcwge4w4gw6dh4zaqndhq";

export class VaultContext extends Context {
  get ContextKey() {
    return "@k2/1password:vault";
  }

  constructor(public readonly vaultId: string) {
    super();
  }
}

export function withDefaultVault() {
  return VaultContext.with(VAULT_ID);
}
