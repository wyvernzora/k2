import { App, HelmCharts, Namespace } from "@k2/cdk-lib";
import { K2Issuer, K2Certificate } from "@k2/cert-manager";
import { K2Secret } from "@k2/1password";

export default {
  create(app: App) {
    const CertManager = HelmCharts.of(app).asChart("cert-manager");

    const chart = new CertManager(app, "cert-manager", {
      ...Namespace.of(app),
      values: {
        extraArgs: ["--controllers=*"],
      },
    });

    const credentials = new K2Secret(chart, "aws-credentials", {
      itemId: "hxitqr6xcco7g2ne3n7m6kkoqa",
    });
    const issuer = new K2Issuer(chart, "issuer", { credentials });
    new K2Certificate(chart, "cert", { issuer });
  },
};
