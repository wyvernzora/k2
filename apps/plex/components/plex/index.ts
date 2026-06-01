import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { K2Chart, K2Volume } from "@k2/cdk-lib";

import { PlexDeployment } from "./deployment.js";
import { PlexService } from "./service.js";

export class Plex extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new PlexDeployment(this, "deployment", {
      volumes: {
        config: K2Volume.provisionNfs({
          name: "plex-config",
          size: Size.gibibytes(256),
        }),
        databases: K2Volume.replicated({
          name: "plex-databases",
          size: Size.gibibytes(8),
        }),
        series: K2Volume.mountNfs({ path: "/mnt/data/media/anime/series", readOnly: true }),
        features: K2Volume.mountNfs({ path: "/mnt/data/media/anime/features", readOnly: true }),
        transcode: K2Volume.ephemeral({ sizeLimit: Size.gibibytes(32) }),
      },
    });
    new PlexService(this, "service");
  }
}
