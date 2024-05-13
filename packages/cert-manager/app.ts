import { K2App, HelmChart } from "@k2/cdk-lib";
import { OnePasswordItem } from "@k2/1password/crds";
import { ClusterIssuer } from "@k2/cert-manager/crds";

const app = new K2App();
const chart = new HelmChart(app, "cert-manager", {
  namespace: "k2-core",
  chart: "helm:https://charts.jetstack.io/cert-manager@v1.14.5",
  values: {
    installCRDs: true,
  },
});

/**
 * AWS credentials for Route53 access
 */
const credentials = new OnePasswordItem(chart, "aws-credentials", {
  spec: {
    itemPath:
      "vaults/zfsyjjcwge4w4gw6dh4zaqndhq/items/hxitqr6xcco7g2ne3n7m6kkoqa",
  },
});

/**
 * Cluster issues using Let's Encrypt and AWS Route53 DNS01 challenge
 */
new ClusterIssuer(chart, "letsencrypt-prod", {
  metadata: {
    name: "letsencrypt-prod",
  },
  spec: {
    acme: {
      email: "wyvernzora+letsencrypt@gmail.com",
      privateKeySecretRef: {
        name: "letsencrypt-prod-privkey",
      },
      server: "https://acme-v02.api.letsencrypt.org/directory",
      solvers: [
        {
          selector: {
            dnsZones: ["wyvernzora.io"],
          },
          dns01: {
            route53: {
              region: "us-west-2",
              accessKeyIdSecretRef: {
                name: credentials.name,
                key: "access-key-id",
              },
              secretAccessKeySecretRef: {
                name: credentials.name,
                key: "secret-access-key",
              },
            },
          },
        },
      ],
    },
  },
});

app.synth();
