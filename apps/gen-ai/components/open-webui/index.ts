import { IngressBackend } from "cdk8s-plus-32";
import { Chart, Size } from "cdk8s";

import { ApexDomain, App, K2Volume, Namespace } from "@k2/cdk-lib";
import { AuthenticatedIngress } from "@k2/auth";

import { OpenWebUIDeployment } from "./deployment.js";

export default {
  create(app: App) {
    const chart = new Chart(app, "open-webui", {
      ...Namespace.of(app),
    });

    const deployment = new OpenWebUIDeployment(chart, "depl", {
      volumes: {
        data: K2Volume.replicated({
          size: Size.gibibytes(4),
        }),
      },
    });
    const service = deployment.exposeViaService({
      ports: [{ port: 80, targetPort: 8080 }],
    });

    new AuthenticatedIngress(chart, "ingr", {
      rules: [
        {
          host: ApexDomain.of(app).subdomain("ai"),
          backend: IngressBackend.fromService(service),
        },
      ],
    });
  },
};
