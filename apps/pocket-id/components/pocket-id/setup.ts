import { fileURLToPath } from "node:url";

import { ApiResource, EnvFieldPaths, EnvValue, Role, RoleBinding, type IServiceAccount } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { ApexDomain, ScriptedJob, type ScriptedJobRbacRule } from "@k2/cdk-lib";
import { FORGEJO_HOST, FORGEJO_NAMESPACE, FORGEJO_OIDC_CLIENT_ID, FORGEJO_OIDC_SECRET_NAME } from "@k2/forgejo";
import { POMERIUM_AUTHENTICATE_HOST_PREFIX, POMERIUM_IDP_SECRET_NAME, POMERIUM_NAMESPACE } from "@k2/pomerium";

import { POCKET_ID_LABELS, POCKET_ID_SERVICE_NAME } from "../../constants.js";

import { STATIC_API_KEY_SECRET_NAME } from "./deployment.js";

const SETUP_JOB_NAME = "setup";
const POMERIUM_RBAC_NAME = "pocket-id-setup";
const FORGEJO_RBAC_NAME = "pocket-id-forgejo-setup";
const SETUP_SCRIPT_PATH = fileURLToPath(new URL("./scripts/setup.py", import.meta.url));

export class PocketIdSetup extends Construct {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const apex = ApexDomain.of(this);
    const setupJob = new ScriptedJob(this, "job", {
      name: SETUP_JOB_NAME,
      script: {
        path: SETUP_SCRIPT_PATH,
        filename: "setup.py",
      },
      env: setupEnv(apex.subdomain(POMERIUM_AUTHENTICATE_HOST_PREFIX)),
      labels: setupLabels(),
      rbacRules: [
        scriptedJobRbacRule(["create", "delete", "get", "patch", "update"], ApiResource.SECRETS),
        scriptedJobRbacRule(["get", "list", "patch", "watch"], ApiResource.DEPLOYMENTS),
      ],
    });

    if (setupJob.serviceAccount === undefined) {
      throw new Error("Pocket ID setup job requires a service account");
    }
    createPomeriumRbac(this, setupJob.serviceAccount);
    createForgejoRbac(this, setupJob.serviceAccount);
  }
}

function createPomeriumRbac(scope: Construct, serviceAccount: IServiceAccount): void {
  const pomeriumRole = new Role(scope, "pomerium-role", {
    metadata: { name: POMERIUM_RBAC_NAME, namespace: POMERIUM_NAMESPACE },
    rules: [{ resources: [ApiResource.SECRETS], verbs: ["create", "get", "patch", "update"] }],
  });
  new RoleBinding(scope, "pomerium-role-binding", {
    metadata: { name: POMERIUM_RBAC_NAME, namespace: POMERIUM_NAMESPACE },
    role: pomeriumRole,
  }).addSubjects(serviceAccount);
}

function createForgejoRbac(scope: Construct, serviceAccount: IServiceAccount): void {
  const forgejoRole = new Role(scope, "forgejo-role", {
    metadata: { name: FORGEJO_RBAC_NAME, namespace: FORGEJO_NAMESPACE },
    rules: [{ resources: [ApiResource.SECRETS], verbs: ["create", "get", "patch", "update"] }],
  });
  new RoleBinding(scope, "forgejo-role-binding", {
    metadata: { name: FORGEJO_RBAC_NAME, namespace: FORGEJO_NAMESPACE },
    role: forgejoRole,
  }).addSubjects(serviceAccount);
}

function setupEnv(authenticateHost: string): Record<string, EnvValue> {
  const authenticateUrl = `https://${authenticateHost}`;
  const forgejoUrl = `https://${FORGEJO_HOST}`;
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
    FORGEJO_NAMESPACE: EnvValue.fromValue(FORGEJO_NAMESPACE),
    FORGEJO_SECRET: EnvValue.fromValue(FORGEJO_OIDC_SECRET_NAME),
    FORGEJO_CLIENT_ID: EnvValue.fromValue(FORGEJO_OIDC_CLIENT_ID),
    FORGEJO_CALLBACK_URL: EnvValue.fromValue(`${forgejoUrl}/user/oauth2/PocketID/callback`),
    FORGEJO_LAUNCH_URL: EnvValue.fromValue(forgejoUrl),
  };
}

function scriptedJobRbacRule(verbs: string[], ...resources: ApiResource[]): ScriptedJobRbacRule {
  return {
    apiGroups: [...new Set(resources.map(resource => resource.apiGroup))],
    resources: resources.map(resource => resource.resourceType),
    verbs,
  };
}

function setupLabels() {
  return {
    ...POCKET_ID_LABELS,
    "app.kubernetes.io/component": "setup",
  };
}
