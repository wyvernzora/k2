import type { Construct } from "constructs";

import { ClusterContext, K2Chart, Namespace } from "@k2/cdk-lib";

import { ClusterSecretStore } from "../crds/external-secrets.io.js";

/**
 * Name + key of the Kubernetes `Secret` that holds the 1Password
 * service-account token. The secret is created OUT-OF-BAND during cluster
 * bootstrap (provisioner CLI -> kairos -> `op item get`); the cdk8s layer
 * does NOT create or manage it. ESO crash-loops until the secret exists.
 *
 *   apiVersion: v1
 *   kind: Secret
 *   metadata:
 *     name:      onepassword-token
 *     namespace: external-secrets
 *   stringData:
 *     token: <1Password service-account token>
 */
const BOOTSTRAP_TOKEN_SECRET_NAME = "onepassword-token";
const BOOTSTRAP_TOKEN_SECRET_KEY = "token";

/**
 * K2-owned ESO provider configuration.
 *
 * Backend: 1Password via service-account token (not Connect). ESO uses the
 * official 1Password Go SDK and authenticates from a pre-provisioned token
 * Secret in this namespace (see BOOTSTRAP_TOKEN_SECRET_NAME above).
 *
 * The ClusterSecretStore is cluster-scoped; every app namespace's
 * ExternalSecret references it as
 * `{ kind: ClusterSecretStore, name: onepassword }`. Vault scope follows
 * `cluster.onePassword.vault`. Add more stores (one per vault) if multi-
 * vault access is ever needed.
 */
export class SecretStores extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);
    const cluster = ClusterContext.of(this).config;
    const namespace = Namespace.of(this).namespace;

    new ClusterSecretStore(this, "onepassword", onePasswordStoreProps(cluster.onePassword.vault, namespace));
  }
}

function onePasswordStoreProps(vault: string, namespace: string) {
  return {
    metadata: { name: "onepassword" },
    spec: onePasswordStoreSpec(vault, namespace),
  };
}

function onePasswordStoreSpec(vault: string, namespace: string) {
  return {
    provider: {
      onepasswordSdk: onePasswordProvider(vault, namespace),
    },
  };
}

function onePasswordProvider(vault: string, namespace: string) {
  return {
    vault,
    auth: onePasswordAuth(namespace),
  };
}

function onePasswordAuth(namespace: string) {
  return {
    serviceAccountSecretRef: onePasswordTokenRef(namespace),
  };
}

function onePasswordTokenRef(namespace: string) {
  return {
    name: BOOTSTRAP_TOKEN_SECRET_NAME,
    key: BOOTSTRAP_TOKEN_SECRET_KEY,
    namespace,
  };
}
