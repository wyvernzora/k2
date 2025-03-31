import { App, HelmChart } from "@k2/cdk-lib";

/**
 * This is a version of the chart that is only used for bootstrapping CRD
 * constructs. The actual application is provisioned at app.ts
 */
const app = new App();
new HelmChart(app, "traefik", {
  namespace: "k2-network",
  chart: "helm:https://traefik.github.io/charts/traefik@34.5.0",
});
app.synth();
