import { Construct } from "constructs";

import { ConfigMap } from "@k2/cdk-lib";

export class ClaudeRemoteConfig extends ConfigMap {
  constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: {
        name: "claude-remote-mcp-config",
      },
    });

    this.addData(".mcp.json", this.renderMcpConfig());
  }

  private renderMcpConfig(): string {
    return JSON.stringify(
      {
        mcpServers: {
          kura: {
            type: "http",
            url: "http://kura-mcp.media.svc.cluster.local/mcp",
          },
          dmhy: {
            type: "http",
            url: "http://dmhy-mcp.media.svc.cluster.local/mcp",
          },
          qbittorrent: {
            type: "http",
            url: "http://qbittorrent-mcp.media.svc.cluster.local/mcp",
          },
        },
      },
      null,
      2,
    );
  }
}
