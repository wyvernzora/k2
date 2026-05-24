import { basename } from "node:path";

import type { IConstruct } from "constructs";

import { Context } from "./base.js";

export class AppRoot extends Context {
  public static readonly contextKey = "k2.appRoot";
  public readonly key = AppRoot.contextKey;
  public readonly appName: string;

  public constructor(public readonly path: string) {
    super();
    this.appName = basename(path);
  }

  public static of(scope: IConstruct): AppRoot {
    return Context.get(scope, AppRoot.contextKey);
  }
}
