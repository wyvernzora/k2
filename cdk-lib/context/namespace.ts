import type { IConstruct } from "constructs";

import { Context } from "./base.js";

export class Namespace extends Context {
  public static readonly contextKey = "k2.namespace";
  public readonly key = Namespace.contextKey;

  public constructor(public readonly namespace: string) {
    super();
  }

  public static of(scope: IConstruct): Namespace {
    return Context.get(scope, Namespace.contextKey);
  }
}
