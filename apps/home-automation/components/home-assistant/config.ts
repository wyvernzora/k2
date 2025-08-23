import { Construct } from "constructs";
import dedent from "dedent-js";

import { ConfigMap } from "@k2/cdk-lib";

export class HomeAssistantConfig extends ConfigMap {
  constructor(scope: Construct, id: string) {
    super(scope, id, {});
    this.addData("init.sh", this.renderInitScript());
    this.addData("configuration.yaml", this.renderConfigurationYaml());
    this.addData("http.yaml", this.renderHttpYaml());
  }

  private renderInitScript(): string {
    return dedent`
      #!/bin/bash

      # Initialize configuration if not present
      if [ ! -f "/config/configuration.yaml" ]; then
        echo "Configuration file not found, copying from templates"
        cp /init/configuration.yaml /config/configuration.yaml
      fi

      # Copy over the HTTP configuration file
      if [ ! -f "/config/http.yaml" ]; then
        echo "HTTP file not found, copying from templates"
        cp /init/http.yaml /config/http.yaml
      fi

      # Check if the automations file exists
      if [ ! -f /config/automations.yaml ]; then
        echo "Automations file not found, creating a new one"
        touch /config/automations.yaml
        echo "[]" >> /config/automations.yaml
      fi

      # Check if the scripts file exists
      if [ ! -f /config/scripts.yaml ]; then
        echo "Scripts file not found, creating a new one"
        touch /config/scripts.yaml
      fi

      # Check if the scenes file exists
      if [ ! -f /config/scenes.yaml ]; then
        echo "Scenes file not found, creating a new one"
        touch /config/scenes.yaml
      fi
      `;
  }

  private renderConfigurationYaml(): string {
    return dedent`
      # Loads default set of integrations. Do not remove.
      default_config:

      # Load frontend themes from the themes folder
      frontend:
        themes: !include_dir_merge_named themes

      http: !include http.yaml
      automation: !include automations.yaml
      script: !include scripts.yaml
      scene: !include scenes.yaml
      `;
  }

  private renderHttpYaml(): string {
    return dedent`
      # Allow reverse proxy
      use_x_forwarded_for: true
      trusted_proxies:
        - 10.42.0.0/16
      `;
  }
}
