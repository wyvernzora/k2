import { Secret } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

const SECRET_NAME = "paperless";
const PROVISIONING_SECRET_NAME = "paperless-provisioning";
const MCP_TOKEN_SECRET_NAME = "paperless-mcp-token";

const PasswordApiVersion = crd.Password.GVK.apiVersion;
const PasswordEncoding = crd.PasswordSpecEncoding;
const ExternalSecretRefreshPolicy = crd.ExternalSecretSpecRefreshPolicy;
const TargetCreationPolicy = crd.ExternalSecretSpecTargetCreationPolicy;
const TargetDeletionPolicy = crd.ExternalSecretSpecTargetDeletionPolicy;
const GeneratorKind = crd.ExternalSecretSpecDataFromSourceRefGeneratorRefKind;

export class PaperlessSecret extends Construct {
  public readonly secretName = SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    newPasswordGenerator(this, "secret-key-generator", "paperless-secret-key", "secretKey", 64);
    newPasswordGenerator(this, "admin-password-generator", "paperless-admin-password", "adminPassword", 32);
    newPasswordGenerator(this, "redis-password-generator", "paperless-redis-password", "redisPassword", 32);

    new crd.ExternalSecret(this, "secret", {
      metadata: { name: SECRET_NAME },
      spec: paperlessExternalSecretSpec(),
    });
  }
}

export class PaperlessProvisioningSecret extends Construct {
  public readonly secretName = PROVISIONING_SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    newPasswordGenerator(this, "mcp-password-generator", "paperless-mcp-password", "mcpPassword", 32);

    new crd.ExternalSecret(this, "secret", {
      metadata: { name: PROVISIONING_SECRET_NAME },
      spec: provisioningExternalSecretSpec(),
    });
  }
}

export class PaperlessMcpTokenSecret extends Secret {
  public readonly secretName = MCP_TOKEN_SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id, {
      metadata: {
        name: MCP_TOKEN_SECRET_NAME,
      },
      type: "Opaque",
    });
  }
}

function newPasswordGenerator(scope: Construct, id: string, name: string, secretKey: string, length: number): void {
  new crd.Password(scope, id, {
    metadata: { name },
    spec: {
      allowRepeat: true,
      digits: 8,
      encoding: PasswordEncoding.RAW,
      length,
      noUpper: false,
      secretKeys: [secretKey],
      symbols: 0,
    },
  });
}

function paperlessExternalSecretSpec(): crd.ExternalSecretSpec {
  return {
    refreshPolicy: ExternalSecretRefreshPolicy.CREATED_ONCE,
    target: {
      creationPolicy: TargetCreationPolicy.OWNER,
      deletionPolicy: TargetDeletionPolicy.RETAIN,
      immutable: true,
      name: SECRET_NAME,
    },
    dataFrom: [
      generatorDataFrom("paperless-secret-key"),
      generatorDataFrom("paperless-admin-password"),
      generatorDataFrom("paperless-redis-password"),
    ],
  };
}

function provisioningExternalSecretSpec(): crd.ExternalSecretSpec {
  return {
    refreshPolicy: ExternalSecretRefreshPolicy.CREATED_ONCE,
    target: {
      creationPolicy: TargetCreationPolicy.OWNER,
      deletionPolicy: TargetDeletionPolicy.RETAIN,
      immutable: true,
      name: PROVISIONING_SECRET_NAME,
    },
    dataFrom: [generatorDataFrom("paperless-mcp-password")],
  };
}

function generatorDataFrom(name: string): crd.ExternalSecretSpecDataFrom {
  return {
    sourceRef: {
      generatorRef: {
        apiVersion: PasswordApiVersion,
        kind: GeneratorKind.PASSWORD,
        name,
      },
    },
  };
}
