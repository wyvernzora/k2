import { Size } from "cdk8s";
import type { Construct } from "constructs";

import { ApexDomain, K2Chart, K2Volume } from "@k2/cdk-lib";
import { PublicIngress } from "@k2/pomerium";

import { HomeAssistantConfig } from "./config.js";
import { HomeAssistantDeployment } from "./deployment.js";
import { HOME_ASSISTANT_SERVICE_NAME } from "./labels.js";
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
        config: K2Volume.replicated({ name: "home-assistant-config", size: Size.gibibytes(1) }),
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
