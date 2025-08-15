import { App, HelmChart } from "@k2/cdk-lib";
import * as OnePassword from "@k2/1password";
import { K2Issuer, K2Certificate } from "@k2/cert-manager";

const app = new App(OnePassword.withDefaultVault());

// Reflector chart to copy secrets across namespaces
new HelmChart(app, "reflector", {
  namespace: "k2-core",
  chart: "helm:https://emberstack.github.io/helm-charts/reflector@9.1.25",
  values: {
    priorityClassName: "system-cluster-critical",
  },
});

// Cert Manager chart
const chart = new HelmChart(app, "cert-manager", {
  namespace: "k2-core",
  chart: "helm:https://charts.jetstack.io/cert-manager@v1.18.2",
  values: {
    installCRDs: true,
    extraArgs: ["--controllers=*"],
  },
});

/**
 * Cluster issues using Let's Encrypt and AWS Route53 DNS01 challenge
 */
const issuer = new K2Issuer(chart, "issuer", {
  credentials: new OnePassword.K2Secret(chart, "aws-credentials", {
    itemId: "hxitqr6xcco7g2ne3n7m6kkoqa",
  }),
});

/**
 * Default certificate instance
 */
new K2Certificate(chart, "cert", { issuer });

app.synth();
