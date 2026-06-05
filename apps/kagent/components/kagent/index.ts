import type { Construct } from "constructs";

import { ApexDomain, HelmCharts, K2Chart } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";

import { KAGENT_UI_PORT } from "../../constants.js";

import { KagentDatabase } from "./database.js";

const KAGENT_DB_VOLUME_NAME = "kagent-db";
const KAGENT_HOST_PREFIX = "kagent";
const KAGENT_UI_SERVICE_NAME = "kagent-ui";
const KAGENT_OPENAI_SECRET_NAME = "kagent-openai";
const KAGENT_DATABASE_CREDENTIALS_SECRET_NAME = "kagent-credentials";
const KAGENT_DATABASE_URL_MOUNT_PATH = "/var/run/kagent-db";
const KAGENT_DATABASE_URL_FILE = `${KAGENT_DATABASE_URL_MOUNT_PATH}/uri`;
const OPENAI_API_KEY_SECRET_KEY = "OPENAI_API_KEY";
const OPENAI_SECRET_SOURCE = "OpenAI API";
const OPENAI_SECRET_SOURCE_FIELD = "credential";

export class Kagent extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new ManagedSecret(this, "openai-secret", {
      metadata: { name: KAGENT_OPENAI_SECRET_NAME },
      secret: OPENAI_SECRET_SOURCE,
      fields: { [OPENAI_API_KEY_SECRET_KEY]: OPENAI_SECRET_SOURCE_FIELD },
    });
    new KagentDatabase(this, "database");
    HelmCharts.of(this).asChart(this, "kagent", "kagent", kagentValues());
    new AuthenticatedIngress(this, "ingress", {
      host: ApexDomain.of(this).subdomain(KAGENT_HOST_PREFIX),
      serviceName: KAGENT_UI_SERVICE_NAME,
      servicePort: KAGENT_UI_PORT,
      policy: authenticatedSourceIpPolicy(),
    });
  }
}

function kagentValues() {
  return {
    "argo-rollouts-agent": { enabled: false },
    "cilium-debug-agent": { enabled: false },
    "cilium-manager-agent": { enabled: false },
    "cilium-policy-agent": { enabled: false },
    "helm-agent": { enabled: false },
    "istio-agent": { enabled: false },
    "k8s-agent": { enabled: false },
    "kgateway-agent": { enabled: false },
    "observability-agent": { enabled: false },
    "promql-agent": { enabled: false },
    "grafana-mcp": { enabled: false },
    querydoc: { enabled: false },
    "kagent-tools": { enabled: false },
    "oauth2-proxy": { enabled: false },
    database: databaseValues(),
    controller: {
      volumes: [databaseUrlVolume()],
      volumeMounts: [databaseUrlVolumeMount()],
    },
    providers: providerValues(),
    ui: {
      backendInternalUrl: "http://kagent-controller:8083/api",
      publicBackendUrl: "/api",
    },
  };
}

function databaseValues() {
  return {
    postgres: postgresDatabaseValues(),
  };
}

function postgresDatabaseValues() {
  return {
    bundled: { enabled: false },
    urlFile: KAGENT_DATABASE_URL_FILE,
    vectorEnabled: false,
  };
}

function providerValues() {
  return {
    default: "openAI",
    openAI: openAiProviderValues(),
  };
}

function openAiProviderValues() {
  return {
    apiKeySecretRef: KAGENT_OPENAI_SECRET_NAME,
    apiKeySecretKey: OPENAI_API_KEY_SECRET_KEY,
    model: "gpt-5.5",
  };
}

function databaseUrlVolume() {
  return {
    name: KAGENT_DB_VOLUME_NAME,
    secret: {
      secretName: KAGENT_DATABASE_CREDENTIALS_SECRET_NAME,
      items: [{ key: "uri", path: "uri" }],
    },
  };
}

function databaseUrlVolumeMount() {
  return {
    name: KAGENT_DB_VOLUME_NAME,
    mountPath: KAGENT_DATABASE_URL_MOUNT_PATH,
    readOnly: true,
  };
}
