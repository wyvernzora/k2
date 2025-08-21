import { App, HelmChart } from "@k2/cdk-lib";
import * as authelia from "@k2/auth";

const app = new App();
new HelmChart(app, "longhorn", {
  namespace: "k2-storage",
  chart: "helm:https://charts.longhorn.io/longhorn@1.9.1",
  values: {
    ingress: {
      enabled: true,
      host: "lh.wyvernzora.io",
      annotations: {
        ...authelia.MiddlewareAnnotation,
      },
    },
  },
});
app.synth();
