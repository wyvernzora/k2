import { createHash } from "node:crypto";

import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

export const RELEASE_INDEXER_CONFIG_KEY = "release-indexer.toml";

const CONFIG_MAP_NAME = "kura-release-indexer-config";

export class ReleaseIndexerConfig extends ConfigMap {
  public readonly checksum: string;

  public constructor(scope: Construct, id: string) {
    const config = renderReleaseIndexerConfig();
    super(scope, id, {
      metadata: { name: CONFIG_MAP_NAME },
      data: {
        [RELEASE_INDEXER_CONFIG_KEY]: config,
      },
    });
    this.checksum = createHash("sha256").update(config).digest("hex");
  }
}

function renderReleaseIndexerConfig(): string {
  return dedent`
    [database]
    schema = "releases"

    [server]
    addr = ":8080"
    metrics_addr = ":9090"
    log_level = "info"

    [queue]
    max_attempts = 3

    [sources.dmhy]
    enabled = true
    interval = "5m"
    settle_window = "24h"
    timeout = "10m"
    request_timeout = "180s"
    url = "https://share.dmhy.org"
    category = "2"
    max_rps = 0.5
    cache_ttl = "5m"

    [sources.nyaa]
    enabled = true
    interval = "5m"
    settle_window = "24h"
    timeout = "2m"
    request_timeout = "30s"
    url = "https://nyaa.si"
    query = ""
    category = "1_4"
    filter = "0"
    max_rps = 0.5
    cache_ttl = "5m"
  `;
}
