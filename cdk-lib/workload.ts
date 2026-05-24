import { Deployment, type DeploymentProps } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import type { K2Mounters, K2Volumes } from "./volumes/base.js";

export class K2Deployment extends Deployment {
  public constructor(scope: Construct, id: string, props: DeploymentProps = {}) {
    super(scope, id, props);
  }

  public attachVolumes<V extends K2Volumes>(volumes: V): K2Mounters<V> {
    const out = {} as K2Mounters<V>;
    for (const [name, volume] of Object.entries(volumes)) {
      const materialized = volume.materialize(this, `vol-${name}`);
      materialized.configureWorkload(this);
      out[name as keyof V] = (path, opts) => ({
        volume: materialized.volume,
        path,
        ...opts,
      });
    }
    return out;
  }
}
