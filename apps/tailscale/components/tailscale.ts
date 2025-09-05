import { K2Secret } from "@k2/1password";
import { HelmCharts, Namespace, App } from "@k2/cdk-lib";

export default {
  create(app: App) {
    const helm = HelmCharts.of(app);
    const TailscaleOperator = helm.asChart("tailscale-operator");

    const chart = new TailscaleOperator(app, "tailscale-op", {
      ...Namespace.of(app),
      values: {},
    });

    new K2Secret(chart, "operator-oauth", {
      metadata: {
        name: "operator-oauth",
      },
      itemId: "4of6ip5wf5s4s5lj2z4bwxww7a",
    });
  },
};
