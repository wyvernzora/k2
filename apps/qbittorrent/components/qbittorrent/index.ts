import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { AuthenticatedIngress, AuthenticatedMcpIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { FLOOD_SERVICE_NAME, QBITTORRENT_MCP_SERVICE_NAME } from "../../constants.js";

import { QbittorrentDeployment } from "./deployment.js";
import { FloodService, QbittorrentMcpService, QbittorrentService } from "./service.js";

const DOWNLOAD_HOST_PREFIX = "dl";

export class Qbittorrent extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const host = ApexDomain.of(this).subdomain(DOWNLOAD_HOST_PREFIX);

    new QbittorrentDeployment(this, "deployment", {
      volumes: {
        appdata: K2Volume.replicated({ size: Size.gibibytes(4) }),
        default: K2Volume.mountNfs({ path: "/mnt/data/downloads" }),
        anime: K2Volume.mountNfs({ path: "/mnt/data/media/anime" }),
      },
    });
    new FloodService(this, "flood-service");
    new QbittorrentService(this, "qbittorrent-service");
    new QbittorrentMcpService(this, "qbittorrent-mcp-service");
    new AuthenticatedIngress(this, "flood-ingress", {
      host,
      serviceName: FLOOD_SERVICE_NAME,
      servicePort: "http",
      policy: authenticatedSourceIpPolicy(),
    });
    new AuthenticatedMcpIngress(this, "mcp-ingress", {
      host,
      path: "/mcp",
      mcpPath: "/mcp",
      serviceName: QBITTORRENT_MCP_SERVICE_NAME,
      servicePort: "mcp",
      policy: authenticatedSourceIpPolicy(),
    });
  }
}
