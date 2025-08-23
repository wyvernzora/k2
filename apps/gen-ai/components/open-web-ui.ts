import { ApexDomain, HelmCharts, Namespace, App } from "@k2/cdk-lib";
import * as Auth from "@k2/auth";

export default {
  create(app: App) {
    const helm = HelmCharts.of(app);
    const OpenWebUI = helm.asChart("open-webui");

    new OpenWebUI(app, "open-webui", {
      ...Namespace.of(app),
      values: {
        ollama: {
          persistence: {
            enabled: false,
            storageClass: "longhorn",
          },
        },
        webui: {
          persistence: {
            storageClass: "longhorn",
          },
          ingress: {
            enabled: true,
            host: ApexDomain.of(app).subdomain("ai"),
            annotations: {
              ...Auth.MiddlewareAnnotation,
            },
          },
        },
      },
    });
  },
};
