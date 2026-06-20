import { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

const SECRET_NAME = "n8n";
const ENCRYPTION_KEY_GENERATOR_NAME = "n8n-encryption-key";
const USER_MANAGEMENT_SECRET_NAME = "n8n-user-management";
const USER_MANAGEMENT_JWT_SECRET_GENERATOR_NAME = "n8n-user-management-jwt-secret";

const PasswordApiVersion = crd.Password.GVK.apiVersion;
const PasswordEncoding = crd.PasswordSpecEncoding;
const ExternalSecretRefreshPolicy = crd.ExternalSecretSpecRefreshPolicy;
const TargetCreationPolicy = crd.ExternalSecretSpecTargetCreationPolicy;
const TargetDeletionPolicy = crd.ExternalSecretSpecTargetDeletionPolicy;
const GeneratorKind = crd.ExternalSecretSpecDataFromSourceRefGeneratorRefKind;

export class N8NSecret extends Construct {
  public readonly secretName = SECRET_NAME;
  public readonly userManagementSecretName = USER_MANAGEMENT_SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new crd.Password(this, "encryption-key-generator", {
      metadata: { name: ENCRYPTION_KEY_GENERATOR_NAME },
      spec: {
        allowRepeat: true,
        digits: 8,
        encoding: PasswordEncoding.RAW,
        length: 32,
        noUpper: false,
        secretKeys: ["encryptionKey"],
        symbols: 0,
      },
    });

    new crd.ExternalSecret(this, "encryption-key", {
      metadata: { name: SECRET_NAME },
      spec: encryptionKeyExternalSecretSpec(),
    });

    new crd.Password(this, "user-management-jwt-secret-generator", {
      metadata: { name: USER_MANAGEMENT_JWT_SECRET_GENERATOR_NAME },
      spec: {
        allowRepeat: true,
        digits: 16,
        encoding: PasswordEncoding.RAW,
        length: 64,
        noUpper: false,
        secretKeys: ["jwtSecret"],
        symbols: 0,
      },
    });

    new crd.ExternalSecret(this, "user-management-jwt-secret", {
      metadata: { name: USER_MANAGEMENT_SECRET_NAME },
      spec: userManagementJwtExternalSecretSpec(),
    });
  }
}

function encryptionKeyExternalSecretSpec(): crd.ExternalSecretSpec {
  return {
    refreshPolicy: ExternalSecretRefreshPolicy.CREATED_ONCE,
    target: {
      creationPolicy: TargetCreationPolicy.OWNER,
      deletionPolicy: TargetDeletionPolicy.RETAIN,
      immutable: true,
      name: SECRET_NAME,
    },
    dataFrom: [encryptionKeyDataFrom()],
  };
}

function userManagementJwtExternalSecretSpec(): crd.ExternalSecretSpec {
  return {
    refreshPolicy: ExternalSecretRefreshPolicy.CREATED_ONCE,
    target: {
      creationPolicy: TargetCreationPolicy.OWNER,
      deletionPolicy: TargetDeletionPolicy.RETAIN,
      immutable: true,
      name: USER_MANAGEMENT_SECRET_NAME,
    },
    dataFrom: [userManagementJwtDataFrom()],
  };
}

function encryptionKeyDataFrom(): crd.ExternalSecretSpecDataFrom {
  return {
    sourceRef: {
      generatorRef: encryptionKeyGeneratorRef(),
    },
  };
}

function userManagementJwtDataFrom(): crd.ExternalSecretSpecDataFrom {
  return {
    sourceRef: {
      generatorRef: userManagementJwtGeneratorRef(),
    },
  };
}

function encryptionKeyGeneratorRef(): crd.ExternalSecretSpecDataFromSourceRefGeneratorRef {
  return {
    apiVersion: PasswordApiVersion,
    kind: GeneratorKind.PASSWORD,
    name: ENCRYPTION_KEY_GENERATOR_NAME,
  };
}

function userManagementJwtGeneratorRef(): crd.ExternalSecretSpecDataFromSourceRefGeneratorRef {
  return {
    apiVersion: PasswordApiVersion,
    kind: GeneratorKind.PASSWORD,
    name: USER_MANAGEMENT_JWT_SECRET_GENERATOR_NAME,
  };
}
