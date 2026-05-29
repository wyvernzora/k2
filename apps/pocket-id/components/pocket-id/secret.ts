import { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

const SECRET_NAME = "pocket-id";

const PasswordApiVersion = crd.Password.GVK.apiVersion;
const PasswordEncoding = crd.PasswordSpecEncoding;
const ExternalSecretRefreshPolicy = crd.ExternalSecretSpecRefreshPolicy;
const GeneratorKind = crd.ExternalSecretSpecDataFromSourceRefGeneratorRefKind;

export class PocketIdSecret extends Construct {
  public readonly secretName = SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new crd.Password(this, "encryption-key-generator", {
      metadata: { name: "pocket-id-encryption-key" },
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
  }
}

function encryptionKeyExternalSecretSpec(): crd.ExternalSecretSpec {
  return {
    refreshPolicy: ExternalSecretRefreshPolicy.CREATED_ONCE,
    target: {
      immutable: true,
      name: SECRET_NAME,
    },
    dataFrom: [encryptionKeyDataFrom()],
  };
}

function encryptionKeyDataFrom(): crd.ExternalSecretSpecDataFrom {
  return {
    sourceRef: {
      generatorRef: encryptionKeyGeneratorRef(),
    },
  };
}

function encryptionKeyGeneratorRef(): crd.ExternalSecretSpecDataFromSourceRefGeneratorRef {
  return {
    apiVersion: PasswordApiVersion,
    kind: GeneratorKind.PASSWORD,
    name: "pocket-id-encryption-key",
  };
}
