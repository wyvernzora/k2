import type { IConstruct } from "constructs";

import { Context } from "./base.js";

export class NfsContext extends Context {
  public static readonly contextKey = "k2.nfs";
  public readonly key = NfsContext.contextKey;

  public constructor(
    public readonly server: string,
    public readonly zone?: string,
  ) {
    super();
  }

  public static of(scope: IConstruct): NfsContext {
    return Context.get(scope, NfsContext.contextKey);
  }
}
