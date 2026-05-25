import type { Construct } from "constructs";

import { HelmCharts, K2Chart, Scheduling } from "@k2/cdk-lib";

import { TlsStore } from "../crds/traefik.io.js";

const DEFAULT_CERTIFICATE_SECRET_NAME = "default-certificate";
type SchedulingProfile = ReturnType<typeof Scheduling.workersPreferred>;

export class Traefik extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const scheduling = Scheduling.workersPreferred();

    const chart = HelmCharts.of(this).asChart(this, "traefik", "traefik", traefikValues(scheduling));

    new TlsStore(chart, "default-tls-store", {
      metadata: {
        name: "default",
      },
      spec: {
        defaultCertificate: {
          secretName: DEFAULT_CERTIFICATE_SECRET_NAME,
        },
      },
    });
  }
}

function traefikValues(scheduling: SchedulingProfile) {
  return {
    ingressClass: {
      enabled: true,
      isDefaultClass: true,
    },
    providers: traefikProviders(),
    priorityClassName: "system-cluster-critical",
    tolerations: scheduling.tolerations,
    affinity: scheduling.affinity,
    service: traefikService(),
    ingressRoute: traefikIngressRoute(),
  };
}

function traefikProviders() {
  return {
    kubernetesCRD: kubernetesCrdProvider(),
    kubernetesIngress: kubernetesIngressProvider(),
    kubernetesGateway: {
      enabled: false,
    },
  };
}

function kubernetesCrdProvider() {
  return {
    enabled: true,
    allowCrossNamespace: true,
  };
}

function kubernetesIngressProvider() {
  return {
    enabled: true,
    publishedService: {
      enabled: true,
    },
  };
}

function traefikService() {
  return {
    spec: {
      ipFamilyPolicy: "PreferDualStack",
    },
  };
}

function traefikIngressRoute() {
  return {
    dashboard: {
      enabled: false,
    },
  };
}
