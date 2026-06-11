import { createHash } from "node:crypto";

import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";
import * as YAML from "yaml";

import { ClientGroup } from "./client-group.js";
import { K8S_GATEWAY_UPSTREAM, K8S_GATEWAY_ZONE } from "./defaults.js";

const CONFIG_MAP_NAME = "blocky-config";
const CONFIG_KEY = "blocky.yaml";

export interface BlockyConfigProps {
  readonly clientGroups: ClientGroup[];
}

export class BlockyConfig extends ConfigMap {
  public readonly checksum: string;

  public constructor(scope: Construct, id: string, props: BlockyConfigProps) {
    const config = renderBlockyConfig(props.clientGroups);
    super(scope, id, {
      metadata: {
        name: CONFIG_MAP_NAME,
      },
      data: {
        [CONFIG_KEY]: config,
      },
    });
    this.checksum = createHash("sha256").update(config).digest("hex");
  }
}

function renderBlockyConfig(clientGroups: ClientGroup[]): string {
  return YAML.stringify(blockyConfig(clientGroups));
}

function blockyConfig(clientGroups: ClientGroup[]) {
  return {
    log: {
      level: "info",
    },
    upstreams: renderUpstreams(clientGroups),
    conditional: renderConditionalForwarding(),
    blocking: renderBlocking(clientGroups),
    ports: {
      dns: 53,
      http: 4000,
    },
    prometheus: {
      enable: true,
      path: "/metrics",
    },
  };
}

function renderConditionalForwarding() {
  return {
    fallbackUpstream: false,
    mapping: {
      [K8S_GATEWAY_ZONE]: K8S_GATEWAY_UPSTREAM,
    },
  };
}

function renderBlocking(clientGroups: ClientGroup[]) {
  const whiteLists: Record<string, string[]> = {};
  const blackLists: Record<string, string[]> = {};
  const clientGroupsBlock: Record<string, string[]> = {};

  for (const clientGroup of clientGroups) {
    for (const blockingGroup of clientGroup.blockingGroups) {
      blackLists[blockingGroup.name] ??= blockingGroup.blacklists;
      whiteLists[blockingGroup.name] ??= blockingGroup.whitelists;
    }
    clientGroupsBlock[clientGroup.name] = clientGroup.blockingGroups.map(group => group.name);
  }

  return {
    whiteLists,
    blackLists,
    clientGroupsBlock,
  };
}

function renderUpstreams(clientGroups: ClientGroup[]) {
  const groups: Record<string, string[]> = {};
  for (const group of clientGroups) {
    groups[group.name] = group.upstream;
  }
  return { groups };
}
