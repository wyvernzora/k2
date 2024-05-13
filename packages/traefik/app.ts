import { K2App, HelmChart } from "@k2/cdk-lib";
import { Certificate } from "@k2/cert-manager/crds";
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

const app = new K2App();
const chart = new HelmChart(app, "traefik", {
  namespace: "k2-network",
  chart: "helm:https://traefik.github.io/charts/traefik@28.0.0",
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
 * Default TLS Certificate
 */
new Certificate(chart, "default-certificate", {
  spec: {
    commonName: "*.wyvernzora.io",
    dnsNames: ["*.wyvernzora.io"],
    issuerRef: {
      kind: "ClusterIssuer",
      name: "letsencrypt-prod",
    },
    secretName: "default-certificate",
    secretTemplate: {
      annotations: {
        "reflector.v1.k8s.emberstack.com/reflection-allowed": "true",
        "reflector.v1.k8s.emberstack.com/reflection-allowed-namespaces":
          "k2-auth,plex",
        "reflector.v1.k8s.emberstack.com/reflection-auto-enabled": "true",
        "reflector.v1.k8s.emberstack.com/reflection-auto-namespace":
          "k2-auth,plex",
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
      secretName: "default-certificate",
    },
  },
});

app.synth();
