import { fileURLToPath } from "node:url";

import { ApiObject, JsonPatch, Size } from "cdk8s";
import {
  Capability,
  Cpu,
  EnvValue,
  ImagePullPolicy,
  PersistentVolumeClaim,
  Secret,
  Volume,
  type ISecret,
  type VolumeMount,
} from "cdk8s-plus-32";
import { Construct } from "constructs";

import { ScriptedJob } from "@k2/cdk-lib";

import { FORGEJO_LABELS, FORGEJO_OIDC_CLIENT_ID, FORGEJO_OIDC_SECRET_NAME } from "../../constants.js";

import { APPDATA_MOUNT_PATH, CONFIG_FILE_PATH, CONFIG_MOUNT_PATH, FORGEJO_IMAGE } from "./deployment.js";
import { forgejoEnv } from "./env.js";

const SETUP_JOB_NAME = "setup";
const SETUP_SCRIPT_PATH = fileURLToPath(new URL("./scripts/setup.sh", import.meta.url));
const OIDC_DISCOVERY_URL = "http://pocket-id.pocket-id.svc.cluster.local/.well-known/openid-configuration";

export interface ForgejoSetupProps {
  readonly appdataClaimName: string;
  readonly credentialsSecretName: string;
  readonly secretName: string;
}

export class ForgejoSetup extends Construct {
  public constructor(scope: Construct, id: string, props: ForgejoSetupProps) {
    super(scope, id);

    const credentialsSecret = Secret.fromSecretName(this, "credentials", props.credentialsSecretName);
    const forgejoSecret = Secret.fromSecretName(this, "secret", props.secretName);
    const oidcSecret = Secret.fromSecretName(this, "oidc-secret", FORGEJO_OIDC_SECRET_NAME);

    const setupJob = new ScriptedJob(this, "job", {
      name: SETUP_JOB_NAME,
      script: { path: SETUP_SCRIPT_PATH, filename: "setup.sh" },
      image: FORGEJO_IMAGE,
      imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
      containerName: "setup",
      env: setupEnv(credentialsSecret, forgejoSecret, oidcSecret),
      labels: setupLabels(),
      mounts: appdataVolumeMounts(this, props),
      resources: {
        cpu: {
          request: Cpu.millis(25),
          limit: Cpu.millis(500),
        },
        memory: {
          request: Size.mebibytes(128),
          limit: Size.mebibytes(512),
        },
      },
      securityContext: {
        user: 1000,
        group: 1000,
        allowPrivilegeEscalation: false,
        capabilities: {
          drop: [Capability.ALL],
        },
        ensureNonRoot: true,
        readOnlyRootFilesystem: false,
      },
    });
    ApiObject.of(setupJob.job).addJsonPatch(
      JsonPatch.add("/spec/template/spec/affinity/podAffinity", setupPodAffinity()),
    );
  }
}

function setupEnv(credentialsSecret: ISecret, forgejoSecret: ISecret, oidcSecret: ISecret): Record<string, EnvValue> {
  return {
    ...forgejoEnv({ credentialsSecret, forgejoSecret }),
    FORGEJO_WORK_PATH: EnvValue.fromValue(APPDATA_MOUNT_PATH),
    FORGEJO_CONFIG_FILE: EnvValue.fromValue(CONFIG_FILE_PATH),
    OIDC_CLIENT_ID: EnvValue.fromValue(FORGEJO_OIDC_CLIENT_ID),
    OIDC_CLIENT_SECRET: oidcSecret.envValue("client_secret"),
    OIDC_DISCOVERY_URL: EnvValue.fromValue(OIDC_DISCOVERY_URL),
  };
}

function appdataVolumeMounts(scope: Construct, props: ForgejoSetupProps): VolumeMount[] {
  const claim = PersistentVolumeClaim.fromClaimName(scope, "setup-appdata-claim", props.appdataClaimName);
  const volume = Volume.fromPersistentVolumeClaim(scope, "setup-appdata-volume", claim);
  return [
    { volume, path: APPDATA_MOUNT_PATH, subPath: "data" },
    { volume, path: CONFIG_MOUNT_PATH, subPath: "config" },
  ];
}

function setupLabels(): Record<string, string> {
  return {
    ...FORGEJO_LABELS,
    "app.kubernetes.io/component": "setup",
  };
}

function setupPodAffinity() {
  return {
    requiredDuringSchedulingIgnoredDuringExecution: [
      {
        labelSelector: {
          matchLabels: FORGEJO_LABELS,
        },
        topologyKey: "kubernetes.io/hostname",
      },
    ],
  };
}
