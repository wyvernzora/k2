import { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

const SECRET_NAME = "forgejo";
const PasswordApiVersion = crd.Password.GVK.apiVersion;
const PasswordEncoding = crd.PasswordSpecEncoding;
const ExternalSecretRefreshPolicy = crd.ExternalSecretSpecRefreshPolicy;
const TargetCreationPolicy = crd.ExternalSecretSpecTargetCreationPolicy;
const TargetDeletionPolicy = crd.ExternalSecretSpecTargetDeletionPolicy;
const GeneratorKind = crd.ExternalSecretSpecDataFromSourceRefGeneratorRefKind;

export class ForgejoSecret extends Construct {
  public readonly secretName = SECRET_NAME;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new crd.Password(this, "secret-key-generator", {
      metadata: { name: "forgejo-secret-key" },
      spec: {
        allowRepeat: true,
        digits: 8,
        encoding: PasswordEncoding.RAW,
        length: 64,
        noUpper: false,
        secretKeys: ["secretKey"],
        symbols: 0,
      },
    });

    new crd.Password(this, "internal-token-generator", {
      metadata: { name: "forgejo-internal-token" },
      spec: {
        allowRepeat: true,
        digits: 8,
        encoding: PasswordEncoding.RAW,
        length: 64,
        noUpper: false,
        secretKeys: ["internalToken"],
        symbols: 0,
      },
    });

    new crd.ExternalSecret(this, "secret", {
      metadata: { name: SECRET_NAME },
      spec: secretExternalSecretSpec(),
    });
  }
}

function secretExternalSecretSpec(): crd.ExternalSecretSpec {
  return {
    refreshPolicy: ExternalSecretRefreshPolicy.CREATED_ONCE,
    target: {
      creationPolicy: TargetCreationPolicy.OWNER,
      deletionPolicy: TargetDeletionPolicy.RETAIN,
      immutable: true,
      name: SECRET_NAME,
    },
    dataFrom: [secretKeyDataFrom(), internalTokenDataFrom()],
  };
}

function secretKeyDataFrom(): crd.ExternalSecretSpecDataFrom {
  return {
    sourceRef: {
      generatorRef: {
        apiVersion: PasswordApiVersion,
        kind: GeneratorKind.PASSWORD,
        name: "forgejo-secret-key",
      },
    },
  };
}

function internalTokenDataFrom(): crd.ExternalSecretSpecDataFrom {
  return {
    sourceRef: {
      generatorRef: {
        apiVersion: PasswordApiVersion,
        kind: GeneratorKind.PASSWORD,
        name: "forgejo-internal-token",
      },
    },
  };
}
