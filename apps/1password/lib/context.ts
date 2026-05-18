import { Context } from "@k2/cdk-lib";

export class VaultContext extends Context {
  get ContextKey() {
    return "@k2/1password:vault";
  }

  constructor(public readonly vaultId: string) {
    super();
  }
}

export function withVault(vaultId: string) {
  return VaultContext.with(vaultId);
}
