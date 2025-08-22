import { AppResourceFunc, ArgoCDResourceFunc, HelmChart, Toleration } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

/* Export raw CRDs */
import * as CRD from "./crds/traefik.io";
import * as GatewayCRD from "./crds/gateway.networking.k8s.io";
import * as HubCRD from "./crds/hub.traefik.io";
import { TlsStore } from "./crds/traefik.io";
import { K2Certificate } from "@k2/cert-manager";

export const crd = {
  ...CRD,
  gateway: GatewayCRD,
  hub: HubCRD,
};

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  const chart = new HelmChart(app, "traefik", {
    namespace: "k2-network",
    chart: "helm:https://traefik.github.io/charts/traefik@37.0.0",
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
      tolerations: [...Toleration.ALLOW_CONTROL_PLANE, ...Toleration.ALLOW_CRITICAL_ADDONS_ONLY],
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
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "traefik", { namespace: "k2-network" });
};
