import { fileURLToPath } from "node:url";

import { Duration, Size } from "cdk8s";
import {
  ApiResource,
  Capability,
  ConfigMap,
  Cpu,
  EnvFieldPaths,
  EnvValue,
  ImagePullPolicy,
  Job,
  RestartPolicy,
  Role,
  RoleBinding,
  ServiceAccount,
  Volume,
  type ContainerProps,
  type IServiceAccount,
} from "cdk8s-plus-32";
import { Construct } from "constructs";

import { ApexDomain, Scheduling } from "@k2/cdk-lib";
import { POMERIUM_AUTHENTICATE_HOST_PREFIX, POMERIUM_IDP_SECRET_NAME, POMERIUM_NAMESPACE } from "@k2/pomerium";

import { POCKET_ID_LABELS, POCKET_ID_NAMESPACE, POCKET_ID_SERVICE_NAME } from "../../lib/constants.js";

import { STATIC_API_KEY_SECRET_NAME } from "./deployment.js";

const SETUP_NAME = "pocket-id-setup";
const SETUP_SCRIPT_CONFIG_MAP = "pocket-id-setup-script";
const SETUP_SCRIPT_PATH = fileURLToPath(new URL("./setup.sh", import.meta.url));
const JOB_RUNNER_IMAGE = "ghcr.io/wyvernzora/k2-job-runner:latest";

export class PocketIdSetup extends Construct {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const apex = ApexDomain.of(this);
    const script = new ConfigMap(this, "script", {
      metadata: { name: SETUP_SCRIPT_CONFIG_MAP },
    });
    script.addFile(SETUP_SCRIPT_PATH, "setup.sh");

    const serviceAccount = new ServiceAccount(this, "service-account", {
      metadata: { name: SETUP_NAME },
    });

    createRbac(this, serviceAccount);

    createSetupJob(this, {
      authenticateHost: apex.subdomain(POMERIUM_AUTHENTICATE_HOST_PREFIX),
      script,
      serviceAccount,
    });
  }
}

interface PocketIdSetupJobProps {
  readonly authenticateHost: string;
  readonly script: ConfigMap;
  readonly serviceAccount: IServiceAccount;
}

function createRbac(scope: Construct, serviceAccount: IServiceAccount): void {
  const pocketIdRole = new Role(scope, "pocket-id-role", {
    metadata: { name: SETUP_NAME },
    rules: [
      rbacRule(["create", "delete", "get", "patch", "update"], ApiResource.SECRETS),
      rbacRule(["get", "list", "patch", "watch"], ApiResource.DEPLOYMENTS),
    ],
  });
  new RoleBinding(scope, "pocket-id-role-binding", {
    metadata: { name: "pocket-id-role-binding" },
    role: pocketIdRole,
  }).addSubjects(serviceAccount);

  const pomeriumRole = new Role(scope, "pomerium-role", {
    metadata: { name: SETUP_NAME, namespace: POMERIUM_NAMESPACE },
    rules: [rbacRule(["create", "get", "patch", "update"], ApiResource.SECRETS)],
  });
  new RoleBinding(scope, "pomerium-role-binding", {
    metadata: { name: SETUP_NAME, namespace: POMERIUM_NAMESPACE },
    role: pomeriumRole,
  }).addSubjects(
    ServiceAccount.fromServiceAccountName(scope, "pomerium-role-binding-subject", SETUP_NAME, {
      namespaceName: POCKET_ID_NAMESPACE,
    }),
  );
}

function createSetupJob(scope: Construct, props: PocketIdSetupJobProps): void {
  const scriptVolume = Volume.fromConfigMap(scope, "setup-script-volume", props.script, {
    name: "setup-script",
    defaultMode: 365,
  });
  const tmpVolume = Volume.fromEmptyDir(scope, "tmp-volume", "tmp", { sizeLimit: Size.mebibytes(16) });
  const job = new Job(scope, "job", {
    metadata: { name: SETUP_NAME },
    backoffLimit: 6,
    restartPolicy: RestartPolicy.ON_FAILURE,
    ttlAfterFinished: Duration.days(1),
    podMetadata: { labels: setupLabels() },
    automountServiceAccountToken: true,
    enableServiceLinks: false,
    securityContext: {
      group: 65532,
      user: 65532,
      ensureNonRoot: true,
    },
    serviceAccount: props.serviceAccount,
    containers: [setupContainer(props.authenticateHost, scriptVolume, tmpVolume)],
  });
  Scheduling.applyWorkersPreferred(job);
}

function setupContainer(authenticateHost: string, scriptVolume: Volume, tmpVolume: Volume): ContainerProps {
  return {
    name: "setup",
    image: JOB_RUNNER_IMAGE,
    imagePullPolicy: ImagePullPolicy.IF_NOT_PRESENT,
    command: ["/bin/bash", "/scripts/setup.sh"],
    envVariables: setupEnv(authenticateHost),
    resources: {
      cpu: {
        request: Cpu.millis(25),
        limit: Cpu.millis(250),
      },
      memory: {
        request: Size.mebibytes(64),
        limit: Size.mebibytes(256),
      },
    },
    securityContext: {
      allowPrivilegeEscalation: false,
      capabilities: { drop: [Capability.ALL] },
      group: 65532,
      user: 65532,
      ensureNonRoot: true,
      readOnlyRootFilesystem: true,
    },
    volumeMounts: [
      { volume: scriptVolume, path: "/scripts", readOnly: true },
      { volume: tmpVolume, path: "/tmp" },
    ],
  };
}

function setupEnv(authenticateHost: string): Record<string, EnvValue> {
  const authenticateUrl = `https://${authenticateHost}`;
  return {
    POD_NAMESPACE: EnvValue.fromFieldRef(EnvFieldPaths.POD_NAMESPACE),
    POCKET_ID_INTERNAL_URL: EnvValue.fromValue(`http://${POCKET_ID_SERVICE_NAME}`),
    POCKET_ID_DEPLOYMENT: EnvValue.fromValue("pocket-id"),
    POCKET_ID_BOOTSTRAP_SECRET: EnvValue.fromValue(STATIC_API_KEY_SECRET_NAME),
    POMERIUM_NAMESPACE: EnvValue.fromValue(POMERIUM_NAMESPACE),
    POMERIUM_SECRET: EnvValue.fromValue(POMERIUM_IDP_SECRET_NAME),
    POMERIUM_CLIENT_ID: EnvValue.fromValue("pomerium"),
    POMERIUM_CALLBACK_URL: EnvValue.fromValue(`${authenticateUrl}/oauth2/callback`),
    POMERIUM_LAUNCH_URL: EnvValue.fromValue(authenticateUrl),
  };
}

function rbacRule(verbs: string[], ...resources: ApiResource[]) {
  return { resources, verbs };
}

function setupLabels() {
  return {
    ...POCKET_ID_LABELS,
    "app.kubernetes.io/component": "setup",
  };
}
