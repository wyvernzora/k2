import type { IConstruct } from "constructs";

import { Context } from "../context/base.js";

import type { ClusterConfig } from "./config.js";

export class ClusterContext extends Context {
  public static readonly contextKey = "k2.cluster";
  public readonly key = ClusterContext.contextKey;

  public constructor(public readonly config: ClusterConfig) {
    super();
  }

  public static of(scope: IConstruct): ClusterContext {
    return Context.get(scope, ClusterContext.contextKey);
  }
}
