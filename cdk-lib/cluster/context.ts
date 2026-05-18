import { Context } from "../context.js";

import type { ClusterConfig } from "./config.js";

export class ClusterContext extends Context {
  get ContextKey() {
    return "@k2/cluster:context";
  }

  readonly cluster: ClusterConfig;

  constructor(clusterConfig: ClusterConfig) {
    super();
    this.cluster = clusterConfig;
  }
}
