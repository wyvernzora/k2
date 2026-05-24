import { Volume } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Volume, SimpleMaterializedVolume, type K2EphemeralProps, type MaterializedVolume } from "./base.js";

export class K2EphemeralVolume extends K2Volume {
  public constructor(private readonly props: K2EphemeralProps) {
    super();
  }

  public materialize(scope: Construct, id: string): MaterializedVolume {
    const volume = Volume.fromEmptyDir(scope, id, id, {
      sizeLimit: this.props.sizeLimit,
    });
    return new SimpleMaterializedVolume(volume);
  }
}
