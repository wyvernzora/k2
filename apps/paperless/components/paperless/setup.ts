import { fileURLToPath } from "node:url";

import { EnvFieldPaths, EnvValue, Secret, type ISecret } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { ScriptedJob } from "@k2/cdk-lib";

import { PAPERLESS_LABELS, PAPERLESS_SERVICE_NAME } from "../../constants.js";

export const PAPERLESS_SETUP_USER = "paperless-setup";
const SETUP_EMAIL = "paperless-setup@kubernetes.local";
const SETUP_JOB_NAME = "setup";
const SETUP_SCRIPT_PATH = fileURLToPath(new URL("./scripts/setup.py", import.meta.url));
const LEGACY_HUMAN_USER = "wyvernzora@gmail.com";
const HUMAN_USER = "wyvernzora";
const HUMAN_EMAIL = "wyvernzora@gmail.com";
const MCP_USER = "paperless-mcp";
const MCP_GROUP = "paperless-mcp-rw";

export interface PaperlessSetupProps {
  readonly appSecretName: string;
  readonly mcpTokenSecretName: string;
  readonly provisioningSecretName: string;
}

export class PaperlessSetup extends Construct {
  public constructor(scope: Construct, id: string, props: PaperlessSetupProps) {
    super(scope, id);

    const appSecret = Secret.fromSecretName(this, "app-secret", props.appSecretName);
    const provisioningSecret = Secret.fromSecretName(this, "provisioning-secret", props.provisioningSecretName);

    new ScriptedJob(this, "job", {
      name: SETUP_JOB_NAME,
      script: {
        path: SETUP_SCRIPT_PATH,
        filename: "setup.py",
      },
      env: setupEnv(props, appSecret, provisioningSecret),
      labels: setupLabels(),
      rbacRules: [
        {
          apiGroups: [""],
          resources: ["secrets"],
          resourceNames: [props.mcpTokenSecretName],
          verbs: ["get", "patch", "update"],
        },
      ],
    });
  }
}

function setupEnv(
  props: PaperlessSetupProps,
  appSecret: ISecret,
  provisioningSecret: ISecret,
): Record<string, EnvValue> {
  return {
    POD_NAMESPACE: EnvValue.fromFieldRef(EnvFieldPaths.POD_NAMESPACE),
    PAPERLESS_INTERNAL_URL: EnvValue.fromValue(`http://${PAPERLESS_SERVICE_NAME}`),
    PAPERLESS_SETUP_USER: EnvValue.fromValue(PAPERLESS_SETUP_USER),
    PAPERLESS_SETUP_EMAIL: EnvValue.fromValue(SETUP_EMAIL),
    PAPERLESS_ADMIN_PASSWORD: appSecret.envValue("adminPassword"),
    PAPERLESS_LEGACY_ADMIN_USER: EnvValue.fromValue(LEGACY_HUMAN_USER),
    PAPERLESS_HUMAN_USER: EnvValue.fromValue(HUMAN_USER),
    PAPERLESS_HUMAN_EMAIL: EnvValue.fromValue(HUMAN_EMAIL),
    PAPERLESS_MCP_USER: EnvValue.fromValue(MCP_USER),
    PAPERLESS_MCP_GROUP: EnvValue.fromValue(MCP_GROUP),
    PAPERLESS_MCP_PASSWORD: provisioningSecret.envValue("mcpPassword"),
    PAPERLESS_MCP_TOKEN_SECRET: EnvValue.fromValue(props.mcpTokenSecretName),
  };
}

function setupLabels(): Record<string, string> {
  return {
    ...PAPERLESS_LABELS,
    "app.kubernetes.io/component": "setup",
  };
}
