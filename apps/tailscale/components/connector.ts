import { Chart } from "cdk8s";

import { Namespace, App } from "@k2/cdk-lib";

import { Connector } from "../crds/tailscale.com.js";

export default {
  create(app: App) {
    const chart = new Chart(app, "ts-connector", {
      ...Namespace.of(app),
    });

    new Connector(chart, "ts-connector", {
      spec: {
        hostname: "k2-net",
        subnetRouter: {
          advertiseRoutes: ["10.0.0.0/8"],
        },
        exitNode: true,
      },
    });
  },
};
