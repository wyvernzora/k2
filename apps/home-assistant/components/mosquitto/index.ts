import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { K2Chart, K2Volume } from "@k2/cdk-lib";

import { MosquittoConfig } from "./config.js";
import { MosquittoDeployment } from "./deployment.js";
import { MosquittoService } from "./service.js";

export class Mosquitto extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const config = new MosquittoConfig(this, "config");
    new MosquittoDeployment(this, "deployment", {
      configName: config.name,
      configChecksum: config.checksum,
      volumes: {
        data: K2Volume.replicated({ name: "mosquitto-data", size: Size.gibibytes(1) }),
        logs: K2Volume.ephemeral({ sizeLimit: Size.gibibytes(1) }),
      },
    });
    new MosquittoService(this, "service");
  }
}
