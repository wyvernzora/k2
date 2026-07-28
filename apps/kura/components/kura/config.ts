import { createHash } from "node:crypto";

import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

export const LIBRARY_MANAGER_CONFIG_KEY = "library-manager.toml";

const CONFIG_MAP_NAME = "kura-library-manager-config";

export class LibraryManagerConfig extends ConfigMap {
  public readonly checksum: string;

  public constructor(scope: Construct, id: string) {
    const config = renderLibraryManagerConfig();
    super(scope, id, {
      metadata: { name: CONFIG_MAP_NAME },
      data: {
        [LIBRARY_MANAGER_CONFIG_KEY]: config,
      },
    });
    this.checksum = createHash("sha256").update(config).digest("hex");
  }
}

function renderLibraryManagerConfig(): string {
  return dedent`
    [server]
    rest = ":8080"
    log_level = "info"
    umask = "0007"

    [library]
    root = "/anime/series"
    inbox = "/anime/downloads"

    [metadata]
    preferred_languages = ["ja"]
  `;
}
