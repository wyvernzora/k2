import { hash } from "crypto";

import { Lazy } from "cdk8s";
import * as base from "cdk8s-plus-32";
import { Construct } from "constructs";
import stringify from "json-stable-stringify";

export class ConfigMap extends base.ConfigMap {
  private readonly checksum: string;

  constructor(scope: Construct, id: string, props: base.ConfigMapProps) {
    super(scope, id, props);
    this.checksum = Lazy.any({
      produce: () => this.computeChecksum(),
    });
  }

  private computeChecksum(): string {
    const data = stringify(this.data);
    const binary = stringify(this.binaryData);
    return hash("sha256", `${data}${binary}`, "hex");
  }

  public addChecksumTo(res: base.Resource): void {
    res.metadata.addAnnotation(`checksum/${this.node.id}`, this.checksum);
  }
}
