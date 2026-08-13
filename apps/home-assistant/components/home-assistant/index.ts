import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { PublicIngress } from "@k2/pomerium";

import { HOME_ASSISTANT_SERVICE_NAME } from "../../constants.js";

import { HomeAssistantConfig } from "./config.js";
import { HomeAssistantDeployment } from "./deployment.js";
import { HomeAssistantService } from "./service.js";

const HOME_ASSISTANT_HOST_PREFIX = "ha";

export class HomeAssistant extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const config = new HomeAssistantConfig(this, "config");
    new HomeAssistantDeployment(this, "deployment", {
      configName: config.name,
      configChecksum: config.checksum,
      volumes: {
        // 1Gi -> 4Gi on the way across: the config volume holds the SQLite
        // recorder database, which grows with state history once devices are
        // actually configured. k2-iscsi allows expansion, so this is sized for
        // what is known rather than guessed at. `from` must keep declaring the
        // live 1Gi.
        config: K2Volume.migrate({
          from: K2Volume.replicated({ name: "home-assistant-config", size: Size.gibibytes(1) }),
          to: K2Volume.iscsi({ size: Size.gibibytes(4) }),
        }),
      },
    });
    new HomeAssistantService(this, "service");
    new PublicIngress(this, "ingress", {
      name: "home-assistant",
      host: ApexDomain.of(this).subdomain(HOME_ASSISTANT_HOST_PREFIX),
      serviceName: HOME_ASSISTANT_SERVICE_NAME,
      servicePort: "http",
    });
  }
}
