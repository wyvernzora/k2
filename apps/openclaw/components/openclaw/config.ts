import { Construct } from "constructs";
import dedent from "dedent-js";

import { ConfigMap } from "@k2/cdk-lib";

export interface OpenClawConfigProps {
  readonly allowedOrigin: string;
}

export class OpenClawConfig extends ConfigMap {
  constructor(scope: Construct, id: string, props: OpenClawConfigProps) {
    super(scope, id, {
      metadata: {
        name: "openclaw-config",
      },
    });

    this.addData("openclaw.json", this.renderOpenClawConfig(props));
    this.addData("AGENTS.md", this.renderAgents());
  }

  private renderOpenClawConfig(props: OpenClawConfigProps): string {
    return JSON.stringify(
      {
        gateway: {
          mode: "local",
          bind: "lan",
          port: 18789,
          trustedProxies: ["10.42.0.0/16"],
          auth: {
            mode: "trusted-proxy",
            trustedProxy: {
              userHeader: "remote-user",
              requiredHeaders: ["x-forwarded-proto", "x-forwarded-host"],
            },
          },
          controlUi: {
            allowedOrigins: [props.allowedOrigin],
          },
          reload: {
            mode: "hybrid",
          },
        },
        agents: {
          defaults: {
            model: {
              primary: "openai/gpt-5.4",
            },
            models: {
              "openai/gpt-5.4": {
                alias: "GPT",
              },
            },
            workspace: "/home/node/.openclaw/workspace",
            userTimezone: "America/Los_Angeles",
          },
        },
      },
      null,
      2,
    );
  }

  private renderAgents(): string {
    return dedent`
      # OpenClaw

      This OpenClaw instance runs inside the K2 homelab Kubernetes cluster.
      Keep actions scoped to the configured workspace unless explicitly directed otherwise.
    `;
  }
}
