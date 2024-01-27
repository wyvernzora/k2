import { ConfigMap } from "cdk8s-plus-27";
import { Construct } from "constructs";
import * as YAML from "yaml";

const DefaultClientGroup = "default";
const DefaultBlocklistGroup = "default";

export interface BlockyConfigProps {
  readonly blockLists: string[];
}

export class BlockyConfig extends ConfigMap {
  constructor(scope: Construct, id: string, props: BlockyConfigProps) {
    super(scope, id, {});
    this.addBlockyConfig(props.blockLists);
  }

  private addBlockyConfig(blockLists: string[]): void {
    this.addData(
      "blocky.yaml",
      YAML.stringify({
        log: {
          level: "debug",
        },
        upstream: {
          [DefaultClientGroup]: [`1.1.1.1`],
        },
        conditional: {
          fallbackUpstream: true,
          mapping: {
            "wyvernzora.io": "k8s-gateway.k2-network.svc.cluster.local",
          },
        },
        blocking: {
          blackLists: {
            [DefaultBlocklistGroup]: blockLists || [],
          },
          clientGroupsBlock: {
            [DefaultClientGroup]: [DefaultBlocklistGroup],
          },
        },
        ports: {
          dns: 53,
          http: 4000,
        },
      }),
    );
  }
}
