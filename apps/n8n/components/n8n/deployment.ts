import { Duration, Size } from "cdk8s";
import {
  Capability,
  Cpu,
  DeploymentStrategy,
  EnvValue,
  ImagePullPolicy,
  LabelSelector,
  Probe,
  Protocol,
  Secret,
  Volume,
  type ContainerProps,
  type ISecret,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";

import { K2Deployment, type K2Mounters, type K2Volumes } from "@k2/cdk-lib";
import * as takuhai from "@k2/takuhai";

import { N8N_HTTP_PORT, N8N_LABELS } from "./labels.js";

const N8N_IMAGE = "n8nio/n8n:2.20.6";
const N8N_PROXY_AUTH_HOOK_IMAGE = "ghcr.io/wyvernzora/n8n-proxy-auth-hook:v0.1.0";
const APPDATA_MOUNT_PATH = "/home/node/.n8n";
const PROXY_AUTH_HOOK_VOLUME_NAME = "proxy-auth-hook";
const PROXY_AUTH_HOOK_INSTALL_PATH = "/out";
const PROXY_AUTH_HOOK_MOUNT_PATH = "/opt/proxy-auth";
const PROXY_AUTH_HOOK_FILE = `${PROXY_AUTH_HOOK_MOUNT_PATH}/hook.cjs`;
const TAKUHAI_NODES_VOLUME_NAME = "takuhai-custom-nodes";
const TAKUHAI_NODES_MOUNT_PATH = "/opt/n8n/custom";
const N8N_HEALTH_PATH = "/n8n-healthz";

export interface N8NDeploymentProps {
  readonly appUrl: string;
  readonly credentialsSecretName: string;
  readonly secretName: string;
  readonly userManagementSecretName: string;
  readonly volumes: K2Volumes;
}

export class N8NDeployment extends K2Deployment {
  public constructor(scope: Construct, id: string, props: N8NDeploymentProps) {
    super(scope, id, {
      metadata: { name: "n8n" },
      replicas: 1,
      select: false,
      strategy: DeploymentStrategy.recreate(),
      podMetadata: { labels: N8N_LABELS },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        ensureNonRoot: true,
        fsGroup: 1000,
      },
    });

    this.select(LabelSelector.of({ labels: N8N_LABELS }));
    const volumes = this.attachVolumes(props.volumes);
    const credentialsSecret = Secret.fromSecretName(this, "credentials-secret", props.credentialsSecretName);
    const n8nSecret = Secret.fromSecretName(this, "n8n-secret", props.secretName);
    const userManagementSecret = Secret.fromSecretName(this, "user-management-secret", props.userManagementSecretName);
    const proxyAuthHookVolume = Volume.fromEmptyDir(this, "proxy-auth-hook-volume", PROXY_AUTH_HOOK_VOLUME_NAME, {
      sizeLimit: Size.mebibytes(1),
    });
    const takuhaiNodesVolume = Volume.fromEmptyDir(this, "takuhai-nodes-volume", TAKUHAI_NODES_VOLUME_NAME, {
      sizeLimit: Size.mebibytes(32),
    });
    const proxyAuthHookInstallMount: VolumeMount = {
      volume: proxyAuthHookVolume,
      path: PROXY_AUTH_HOOK_INSTALL_PATH,
    };
    const proxyAuthHookMount: VolumeMount = {
      volume: proxyAuthHookVolume,
      path: PROXY_AUTH_HOOK_MOUNT_PATH,
      readOnly: true,
    };
    const takuhaiNodesMount: VolumeMount = {
      volume: takuhaiNodesVolume,
      path: TAKUHAI_NODES_MOUNT_PATH,
      readOnly: true,
    };

    this.addInitContainer(proxyAuthHookInitContainer(proxyAuthHookInstallMount));
    this.addInitContainer(
      takuhai.n8nCustomNodesInitContainer({
        volume: takuhaiNodesVolume,
        path: TAKUHAI_NODES_MOUNT_PATH,
        resources: initResources(),
      }),
    );
    this.addContainer(
      n8nContainer(
        props,
        volumes,
        credentialsSecret,
        n8nSecret,
        userManagementSecret,
        proxyAuthHookMount,
        takuhaiNodesMount,
      ),
    );
  }
}

function proxyAuthHookInitContainer(proxyAuthHookMount: VolumeMount): ContainerProps {
  return {
    name: "install-proxy-auth-hook",
    image: N8N_PROXY_AUTH_HOOK_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    volumeMounts: [proxyAuthHookMount],
    resources: initResources(),
    securityContext: {
      capabilities: {
        drop: [Capability.ALL],
      },
      ensureNonRoot: false,
      readOnlyRootFilesystem: true,
    },
  };
}

function n8nContainer(
  props: N8NDeploymentProps,
  volumes: K2Mounters<K2Volumes>,
  credentialsSecret: ISecret,
  n8nSecret: ISecret,
  userManagementSecret: ISecret,
  proxyAuthHookMount: VolumeMount,
  takuhaiNodesMount: VolumeMount,
): ContainerProps {
  const url = new URL(props.appUrl);
  const jwksUrl = new URL("/.well-known/pomerium/jwks.json", props.appUrl);
  return {
    name: "n8n",
    image: N8N_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    ports: [{ name: "http", number: N8N_HTTP_PORT, protocol: Protocol.TCP }],
    envVariables: {
      N8N_PORT: EnvValue.fromValue(String(N8N_HTTP_PORT)),
      N8N_HOST: EnvValue.fromValue(url.host),
      N8N_PROTOCOL: EnvValue.fromValue(url.protocol.replace(":", "")),
      N8N_EDITOR_BASE_URL: EnvValue.fromValue(props.appUrl),
      N8N_DIAGNOSTICS_ENABLED: EnvValue.fromValue("false"),
      N8N_ENCRYPTION_KEY: n8nSecret.envValue("encryptionKey"),
      N8N_ENFORCE_SETTINGS_FILE_PERMISSIONS: EnvValue.fromValue("true"),
      N8N_RUNNERS_ENABLED: EnvValue.fromValue("true"),
      N8N_CUSTOM_EXTENSIONS: EnvValue.fromValue(TAKUHAI_NODES_MOUNT_PATH),
      N8N_ENDPOINT_HEALTH: EnvValue.fromValue(N8N_HEALTH_PATH),
      N8N_USER_MANAGEMENT_JWT_SECRET: userManagementSecret.envValue("jwtSecret"),
      N8N_PROXY_HOPS: EnvValue.fromValue("1"),
      EXTERNAL_HOOK_FILES: EnvValue.fromValue(PROXY_AUTH_HOOK_FILE),
      N8N_PROXY_AUTH_JWKS_URL: EnvValue.fromValue(jwksUrl.toString()),
      N8N_PROXY_AUTH_ISSUER: EnvValue.fromValue(url.host),
      N8N_PROXY_AUTH_AUDIENCE: EnvValue.fromValue(url.host),
      N8N_PROXY_AUTH_ALGORITHMS: EnvValue.fromValue("ES256"),
      DB_TYPE: EnvValue.fromValue("postgresdb"),
      DB_POSTGRESDB_HOST: credentialsSecret.envValue("host"),
      DB_POSTGRESDB_PORT: credentialsSecret.envValue("port"),
      DB_POSTGRESDB_DATABASE: credentialsSecret.envValue("dbname"),
      DB_POSTGRESDB_USER: credentialsSecret.envValue("user"),
      DB_POSTGRESDB_PASSWORD: credentialsSecret.envValue("password"),
      DB_POSTGRESDB_SCHEMA: EnvValue.fromValue("public"),
      DB_POSTGRESDB_SSL_ENABLED: EnvValue.fromValue("false"),
      GENERIC_TIMEZONE: EnvValue.fromValue("America/Los_Angeles"),
      TZ: EnvValue.fromValue("America/Los_Angeles"),
    },
    volumeMounts: [volumes.appdata(APPDATA_MOUNT_PATH), proxyAuthHookMount, takuhaiNodesMount],
    liveness: Probe.fromHttpGet(N8N_HEALTH_PATH, { port: N8N_HTTP_PORT }),
    readiness: Probe.fromHttpGet(`${N8N_HEALTH_PATH}/readiness`, { port: N8N_HTTP_PORT }),
    startup: n8nStartupProbe(),
    resources: {
      cpu: {
        request: Cpu.millis(100),
        limit: Cpu.millis(2000),
      },
      memory: {
        request: Size.mebibytes(512),
        limit: Size.gibibytes(4),
      },
      ephemeralStorage: {
        limit: Size.gibibytes(10),
      },
    },
    securityContext: {
      capabilities: {
        drop: [Capability.ALL],
      },
      user: 1000,
      group: 1000,
      ensureNonRoot: true,
      readOnlyRootFilesystem: false,
    },
  };
}

function initResources(): ContainerProps["resources"] {
  return {
    cpu: {
      request: Cpu.millis(10),
      limit: Cpu.millis(100),
    },
    memory: {
      request: Size.mebibytes(16),
      limit: Size.mebibytes(64),
    },
  };
}

function n8nStartupProbe(): Probe {
  return Probe.fromHttpGet(N8N_HEALTH_PATH, {
    port: N8N_HTTP_PORT,
    failureThreshold: 30,
    periodSeconds: Duration.seconds(10),
    timeoutSeconds: Duration.seconds(5),
  });
}
