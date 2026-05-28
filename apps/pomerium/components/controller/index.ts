import { KubeIngressClass, KubeNamespace, KubeServiceAccount } from "cdk8s-plus-32/lib/imports/k8s.js";
import type { Construct } from "constructs";

import { K2Chart, Scheduling } from "@k2/cdk-lib";

import { POMERIUM_CONTROLLER_NAME, POMERIUM_INGRESS_CLASS_NAME, POMERIUM_NAMESPACE } from "../../lib/constants.js";

import { GEN_SECRETS_SERVICE_ACCOUNT, createRbac } from "./rbac.js";
import { clusterMetadata, metadata } from "./metadata.js";
import { createServices } from "./services.js";
import { createWorkloads } from "./workloads.js";

export class PomeriumController extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const scheduling = Scheduling.workersPreferred();

    new KubeNamespace(this, "namespace", {
      metadata: clusterMetadata(POMERIUM_NAMESPACE),
    });
    new KubeServiceAccount(this, "controller-service-account", {
      metadata: metadata(POMERIUM_CONTROLLER_NAME),
    });
    new KubeServiceAccount(this, "gen-secrets-service-account", {
      metadata: metadata(GEN_SECRETS_SERVICE_ACCOUNT),
    });

    createRbac(this);
    createServices(this);
    createWorkloads(this, scheduling);
    createIngressClass(this);
  }
}

function createIngressClass(scope: Construct) {
  new KubeIngressClass(scope, "ingress-class", {
    metadata: {
      ...clusterMetadata(POMERIUM_INGRESS_CLASS_NAME),
      name: POMERIUM_INGRESS_CLASS_NAME,
      annotations: {
        "ingressclass.kubernetes.io/is-default-class": "true",
      },
    },
    spec: {
      controller: "pomerium.io/ingress-controller",
    },
  });
}
