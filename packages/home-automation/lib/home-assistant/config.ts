import { ConfigMap } from "@k2/cdk-lib";
import { Construct } from "constructs";
import dedent from "dedent-js";

export class HomeAssistantConfig extends ConfigMap {
  constructor(scope: Construct, id: string) {
    super(scope, id, {});
    this.addData("configuration.yaml", this.renderConfigurationYaml());
  }

  private renderConfigurationYaml(): string {
    return dedent`
      # Loads default set of integrations. Do not remove.
      default_config:

      # Allow reverse proxy
      http:
        use_x_forwarded_for: true
        trusted_proxies:
          - 10.42.0.0/16
      `;
  }
}
