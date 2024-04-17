import { ConfigMap } from "cdk8s-plus-28";
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
        customDNS: {
          customTTL: "1h",
          filterUnmappedTypes: true,
          mapping: {
            "unifi.wyvernzora.io": "10.10.1.1",
            "roxy.wyvernzora.io": "10.10.7.1",
            "eris.wyvernzora.io": "10.10.7.2",
            "sylphy.wyvernzora.io": "10.10.7.3",
            "pve.wyvernzora.io": "10.10.7.254",
            "rumi.wyvernzora.io": "10.10.8.1",
            "k8s.wyvernzora.io": "10.10.8.2",
          },
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
