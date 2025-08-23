import { Construct } from "constructs";
import * as YAML from "yaml";

import { ConfigMap } from "@k2/cdk-lib";

import { ClientGroup } from "./client-group.js";
import { CustomDns } from "./custom-dns.js";

export interface BlockyConfigProps {
  readonly apexDomain: string;
  readonly clientGroups: ClientGroup[];
  readonly customDns?: CustomDns;
}

export class BlockyConfig extends ConfigMap {
  constructor(scope: Construct, id: string, props: BlockyConfigProps) {
    super(scope, id, {});
    this.addBlockyConfig(props);
  }

  private addBlockyConfig(props: BlockyConfigProps): void {
    const config = {
      log: {
        level: "debug",
      },
      upstreams: renderUpstreams(props.clientGroups),
      blocking: renderBlocking(props.clientGroups),
      customDNS: renderCustomDns(props.customDns || CustomDns.empty(), props.apexDomain),
    };

    this.addData(
      "blocky.yaml",
      YAML.stringify({
        ...config,
        conditional: {
          fallbackUpstream: true,
          mapping: {
            "wyvernzora.io": "k8s-gateway.dns.svc.cluster.local",
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

function renderUpstreams(clientGroups: ClientGroup[]) {
  const groups: Record<string, string[]> = {};
  for (const group of clientGroups) {
    groups[group.name] = group.upstream;
  }
  return { groups };
}

function renderBlocking(clientGroups: ClientGroup[]) {
  const whiteLists: Record<string, string[]> = {};
  const blackLists: Record<string, string[]> = {};
  const clientGroupsBlock: Record<string, string[]> = {};

  for (const cg of clientGroups) {
    // Add blocking groups
    for (const bg of cg.blockingGroups) {
      if (!blackLists[bg.name]) {
        blackLists[bg.name] = bg.blacklists;
      }
      if (!whiteLists[bg.name]) {
        whiteLists[bg.name] = bg.whitelists;
      }
    }
    // Add client to blocking group mappings
    clientGroupsBlock[cg.name] = cg.blockingGroups.map(bg => bg.name);
  }
  return { whiteLists, blackLists, clientGroupsBlock };
}

function renderCustomDns(customDns: CustomDns, apex: string) {
  const mapping: Record<string, string> = {};
  // eslint-disable-next-line prefer-const
  for (let [host, addrs] of Object.entries(customDns.records)) {
    if (!host.endsWith(apex)) {
      host = `${host}.${apex}`;
    }
    mapping[host] = addrs.join(",");
  }
  return {
    customTTL: `${customDns.ttl.toSeconds()}s`,
    filterUnmappedTypes: true,
    mapping,
  };
}
