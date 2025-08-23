import { App, HelmCharts, Namespace } from "@k2/cdk-lib";

export default {
  create(app: App) {
    const Reflector = HelmCharts.of(app).asChart("reflector");

    new Reflector(app, "reflector", {
      ...Namespace.of(app),
      values: {
        priorityClassName: "system-cluster-critical",
      },
    });
  },
};
