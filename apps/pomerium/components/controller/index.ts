import { Namespace as KubernetesNamespace, ServiceAccount, k8s } from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Chart } from "@k2/cdk-lib";

import { POMERIUM_CONTROLLER_NAME, POMERIUM_INGRESS_CLASS_NAME, POMERIUM_NAMESPACE } from "../../constants.js";

import { GEN_SECRETS_SERVICE_ACCOUNT, createRbac } from "./rbac.js";
import { clusterMetadata, metadata } from "./metadata.js";
import { createServices } from "./services.js";
import { createWorkloads } from "./workloads.js";

export class PomeriumController extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new KubernetesNamespace(this, "namespace", {
      metadata: clusterMetadata(POMERIUM_NAMESPACE),
    });
    const controllerServiceAccount = new ServiceAccount(this, "controller-service-account", {
      metadata: metadata(POMERIUM_CONTROLLER_NAME),
    });
    const genSecretsServiceAccount = new ServiceAccount(this, "gen-secrets-service-account", {
      metadata: metadata(GEN_SECRETS_SERVICE_ACCOUNT),
    });

    createRbac(this, controllerServiceAccount, genSecretsServiceAccount);
    createServices(this);
    createWorkloads(this, controllerServiceAccount, genSecretsServiceAccount);
    createIngressClass(this);
  }
}

function createIngressClass(scope: Construct) {
  // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus does not expose an IngressClass L2 construct.
  new k8s.KubeIngressClass(scope, "ingress-class", {
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
