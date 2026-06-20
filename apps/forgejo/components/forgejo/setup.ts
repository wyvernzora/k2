import { ApiObject, JsonPatch, Size } from "cdk8s";
import {
  Capability,
  Cpu,
  EnvValue,
  ImagePullPolicy,
  Job,
  PersistentVolumeClaim,
  RestartPolicy,
  Secret,
  Volume,
  type ContainerProps,
  type ISecret,
  type VolumeMount,
} from "cdk8s-plus-32";
import type { Construct } from "constructs";
import dedent from "dedent-js";

import { Scheduling, only, workers } from "@k2/cdk-lib";

import { FORGEJO_LABELS } from "../../constants.js";

import { APPDATA_MOUNT_PATH, CONFIG_FILE_PATH, CONFIG_MOUNT_PATH, FORGEJO_IMAGE } from "./deployment.js";
import { forgejoEnv } from "./env.js";

const SETUP_JOB_NAME = "setup";
const OIDC_SECRET_NAME = "forgejo-oidc";
const OIDC_CLIENT_ID = "forgejo";

export interface ForgejoSetupProps {
  readonly appdataClaimName: string;
  readonly credentialsSecretName: string;
  readonly secretName: string;
}

export class ForgejoSetup extends Job {
  public constructor(scope: Construct, id: string, props: ForgejoSetupProps) {
    const credentialsSecret = Secret.fromSecretName(scope, `${id}-credentials`, props.credentialsSecretName);
    const forgejoSecret = Secret.fromSecretName(scope, `${id}-secret`, props.secretName);
    const oidcSecret = Secret.fromSecretName(scope, `${id}-oidc-secret`, OIDC_SECRET_NAME);

    super(scope, id, {
      metadata: { name: SETUP_JOB_NAME },
      restartPolicy: RestartPolicy.ON_FAILURE,
      backoffLimit: 6,
      podMetadata: { labels: setupLabels() },
      automountServiceAccountToken: false,
      enableServiceLinks: false,
      securityContext: {
        group: 1000,
        user: 1000,
        ensureNonRoot: true,
      },
      containers: [setupContainer(credentialsSecret, forgejoSecret, oidcSecret, appdataVolumeMounts(scope, props))],
    });
    Scheduling.of(this).apply(only(workers()));
    ApiObject.of(this).addJsonPatch(JsonPatch.add("/spec/template/spec/affinity/podAffinity", setupPodAffinity()));
  }
}

function setupContainer(
  credentialsSecret: ISecret,
  forgejoSecret: ISecret,
  oidcSecret: ISecret,
  volumeMounts: VolumeMount[],
): ContainerProps {
  return {
    name: "setup",
    image: FORGEJO_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: ["sh", "-c", setupScript()],
    volumeMounts,
    envVariables: {
      ...forgejoEnv({ credentialsSecret, forgejoSecret }),
      OIDC_CLIENT_ID: EnvValue.fromValue(OIDC_CLIENT_ID),
      OIDC_CLIENT_SECRET: oidcSecret.envValue("client_secret"),
      OIDC_DISCOVERY_URL: EnvValue.fromValue("https://id.k2.wyvernzora.io/.well-known/openid-configuration"),
    },
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

function setupScript(): string {
  return dedent`
    set -eu

    forgejo_admin() {
      forgejo --work-path "${APPDATA_MOUNT_PATH}" --config "${CONFIG_FILE_PATH}" admin "$@"
    }

    auth_exists() {
      forgejo_admin auth list 2>/tmp/forgejo-auth-list.err | grep -q "PocketID"
    }

    ready=false
    for _ in $(seq 1 60); do
      if forgejo_admin auth list >/tmp/forgejo-auth-list.out 2>/tmp/forgejo-auth-list.err; then
        ready=true
        break
      fi
      sleep 5
    done

    if [ "$ready" != "true" ]; then
      cat /tmp/forgejo-auth-list.err
      exit 1
    fi

    if auth_exists; then
      echo "PocketID OAuth source already exists"
      exit 0
    fi

    forgejo_admin auth add-oauth \
      --name PocketID \
      --provider openidConnect \
      --key "$OIDC_CLIENT_ID" \
      --secret "$OIDC_CLIENT_SECRET" \
      --auto-discover-url "$OIDC_DISCOVERY_URL" \
      --scopes openid \
      --scopes profile \
      --scopes email
  `;
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
