import type { Construct } from "constructs";

import { ApexDomain, HelmCharts, K2Chart } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";
import * as kura from "@k2/kura";
import { AuthenticatedIngress, authenticatedSourceIpPolicy } from "@k2/pomerium";
import * as qbittorrent from "@k2/qbittorrent";

import { GPT_5_4_MINI_MODEL_CONFIG_NAME, GPT_5_5_MODEL_CONFIG_NAME, KAGENT_UI_PORT } from "../../constants.js";
import {
  ModelConfigV1Alpha2,
  ModelConfigV1Alpha2SpecProvider,
  RemoteMcpServer,
  RemoteMcpServerSpecProtocol,
} from "../../crds/kagent.dev.js";
import type { McpServer } from "../../lib/agent.js";

import { KagentDatabase } from "./database.js";

const Provider = ModelConfigV1Alpha2SpecProvider;
const McpProtocol = RemoteMcpServerSpecProtocol;

const KAGENT_DB_VOLUME_NAME = "kagent-db";
const KAGENT_HOST_PREFIX = "kagent";
const KAGENT_UI_SERVICE_NAME = "kagent-ui";
const KAGENT_DATABASE_CREDENTIALS_SECRET_NAME = "kagent-credentials";
const KAGENT_OPENAI_SECRET_NAME = "kagent-openai";
const KAGENT_DATABASE_URL_MOUNT_PATH = "/var/run/kagent-db";
const KAGENT_DATABASE_URL_FILE = `${KAGENT_DATABASE_URL_MOUNT_PATH}/uri`;
const OPENAI_API_KEY_SECRET_KEY = "OPENAI_API_KEY";
const OPENAI_SECRET_ID = "auxpf6o2uqun2igak64lteb5tq";
const OPENAI_SECRET_FIELD = "credential";

export class KAgentSystem extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new ManagedSecret(this, "openai-secret", {
      metadata: { name: KAGENT_OPENAI_SECRET_NAME },
      secretId: OPENAI_SECRET_ID,
      fields: { [OPENAI_API_KEY_SECRET_KEY]: OPENAI_SECRET_FIELD },
    });
    new OpenAiModelConfig(this, "gpt-5-5", {
      name: GPT_5_5_MODEL_CONFIG_NAME,
      model: "gpt-5.5",
    });
    new OpenAiModelConfig(this, "gpt-5-4-mini", {
      name: GPT_5_4_MINI_MODEL_CONFIG_NAME,
      model: "gpt-5.4-mini",
    });
    for (const server of remoteMcpServers()) {
      new StreamableHttpRemoteMcpServer(this, server.name, server);
    }
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

interface OpenAiModelConfigProps {
  readonly name: string;
  readonly model: string;
}

class OpenAiModelConfig extends ModelConfigV1Alpha2 {
  public constructor(scope: Construct, id: string, props: OpenAiModelConfigProps) {
    super(scope, id, {
      metadata: { name: props.name },
      spec: {
        apiKeySecret: KAGENT_OPENAI_SECRET_NAME,
        apiKeySecretKey: OPENAI_API_KEY_SECRET_KEY,
        model: props.model,
        provider: Provider.OPEN_AI,
      },
    });
  }
}

class StreamableHttpRemoteMcpServer extends RemoteMcpServer {
  public constructor(scope: Construct, id: string, props: McpServer) {
    super(scope, id, {
      metadata: { name: props.name },
      spec: {
        description: props.description,
        protocol: McpProtocol.STREAMABLE_UNDERSCORE_HTTP,
        sseReadTimeout: "5m",
        terminateOnClose: true,
        timeout: "60s",
        url: props.url,
      },
    });
  }
}

function remoteMcpServers(): McpServer[] {
  return [kura.mcpServers.kura(), kura.mcpServers.dmhy(), qbittorrent.mcpServers.qbittorrent()];
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
