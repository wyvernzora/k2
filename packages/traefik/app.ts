import { App, HelmChart } from "@k2/cdk-lib";
import { K2Certificate } from "@k2/cert-manager";
import { TlsStore } from "@k2/traefik/crds";

const TOLERATE_CONTROL_PLANE = {
  tolerations: [
    {
      key: "CriticalAddonsOnly",
      operator: "Exists",
    },
    {
      key: "node-role.kubernetes.io/control-plane",
      operator: "Exists",
      effect: "NoSchedule",
    },
    {
      key: "node-role.kubernetes.io/master",
      operator: "Exists",
      effect: "NoSchedule",
    },
  ],
};

const app = new App();
const chart = new HelmChart(app, "traefik", {
  namespace: "k2-network",
  chart: "helm:https://traefik.github.io/charts/traefik@28.3.0",
  values: {
    podAnnotations: {
      "prometheus.io/port": "8082",
      "prometheus.io/scrape": "true",
    },
    providers: {
      kubernetesCRD: {
        enabled: true,
        allowCrossNamespace: true,
      },
      kubernetesIngress: {
        publishedService: {
          enabled: true,
        },
      },
    },
    priorityClassName: "system-cluster-critical",
    ...TOLERATE_CONTROL_PLANE,
    service: {
      ipFamilyPolicy: "PreferDualStack",
    },
    ingressRoute: {
      dashboard: {
        enabled: true,
        matchRule: "Host(`k2.wyvernzora.io`) && PathPrefix(`/traefik`)",
        entryPoints: ["web", "websecure"],
        middlewares: [
          {
            name: "k2-auth-authelia@kubernetescrd",
          },
        ],
      },
    },
  },
});

/**
 * Default TLS Store
 */
new TlsStore(chart, "default", {
  metadata: {
    name: "default",
  },
  spec: {
    defaultCertificate: {
      secretName: K2Certificate.Name,
    },
  },
});

app.synth();
