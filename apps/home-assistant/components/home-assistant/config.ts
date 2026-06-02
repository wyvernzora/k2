import { createHash } from "node:crypto";

import { ConfigMap } from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

import { ClusterContext } from "@k2/cdk-lib";

const CONFIG_MAP_NAME = "home-assistant-init";

export class HomeAssistantConfig extends ConfigMap {
  public readonly checksum: string;

  public constructor(scope: Construct, id: string) {
    const data = configData(ClusterContext.of(scope).config.kubernetes.subnets.pods);
    super(scope, id, {
      metadata: { name: CONFIG_MAP_NAME },
      data,
    });
    this.checksum = createHash("sha256").update(JSON.stringify(data)).digest("hex");
  }
}

function configData(trustedProxyCidr: string): Record<string, string> {
  return {
    "init.sh": initScript(),
    "configuration.yaml": configurationYaml(),
    "http.yaml": httpYaml(trustedProxyCidr),
  };
}

function initScript(): string {
  return dedent`
    #!/bin/sh
    set -eu

    if [ ! -f /config/configuration.yaml ]; then
      cp /init/configuration.yaml /config/configuration.yaml
    fi

    if [ ! -f /config/http.yaml ]; then
      cp /init/http.yaml /config/http.yaml
    fi

    if [ ! -f /config/automations.yaml ]; then
      printf '[]\\n' > /config/automations.yaml
    fi

    touch /config/scripts.yaml
    touch /config/scenes.yaml
  `;
}

function configurationYaml(): string {
  return dedent`
    default_config:

    frontend:
      themes: !include_dir_merge_named themes

    http: !include http.yaml
    automation: !include automations.yaml
    script: !include scripts.yaml
    scene: !include scenes.yaml
  `;
}

function httpYaml(trustedProxyCidr: string): string {
  return dedent`
    use_x_forwarded_for: true
    trusted_proxies:
      - ${trustedProxyCidr}
  `;
}
