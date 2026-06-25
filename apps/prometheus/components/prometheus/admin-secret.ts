import { Construct } from "constructs";

import { crd } from "@k2/external-secrets";

export const GRAFANA_ADMIN_SECRET_NAME = "prometheus-grafana-admin";

const GRAFANA_ADMIN_PASSWORD_GENERATOR_NAME = "prometheus-grafana-admin-password";
const PasswordApiVersion = crd.Password.GVK.apiVersion;
const PasswordEncoding = crd.PasswordSpecEncoding;
const ExternalSecretRefreshPolicy = crd.ExternalSecretSpecRefreshPolicy;
const GeneratorKind = crd.ExternalSecretSpecDataFromSourceRefGeneratorRefKind;
const TargetCreationPolicy = crd.ExternalSecretSpecTargetCreationPolicy;
const TargetDeletionPolicy = crd.ExternalSecretSpecTargetDeletionPolicy;
const TargetTemplateEngineVersion = crd.ExternalSecretSpecTargetTemplateEngineVersion;
const TargetTemplateMergePolicy = crd.ExternalSecretSpecTargetTemplateMergePolicy;

export class GrafanaAdminSecret extends Construct {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new crd.Password(this, "password-generator", {
      metadata: { name: GRAFANA_ADMIN_PASSWORD_GENERATOR_NAME },
      spec: {
        allowRepeat: true,
        digits: 8,
        encoding: PasswordEncoding.RAW,
        length: 32,
        noUpper: false,
        secretKeys: ["adminPassword"],
        symbols: 0,
      },
    });

    new crd.ExternalSecret(this, "secret", {
      metadata: { name: GRAFANA_ADMIN_SECRET_NAME },
      spec: grafanaAdminExternalSecretSpec(),
    });
  }
}

function grafanaAdminExternalSecretSpec(): crd.ExternalSecretSpec {
  return {
    refreshPolicy: ExternalSecretRefreshPolicy.CREATED_ONCE,
    target: grafanaAdminTarget(),
    dataFrom: [grafanaAdminPasswordDataFrom()],
  };
}

function grafanaAdminTarget(): crd.ExternalSecretSpecTarget {
  return {
    creationPolicy: TargetCreationPolicy.OWNER,
    deletionPolicy: TargetDeletionPolicy.RETAIN,
    immutable: true,
    name: GRAFANA_ADMIN_SECRET_NAME,
    template: {
      engineVersion: TargetTemplateEngineVersion.V2,
      mergePolicy: TargetTemplateMergePolicy.REPLACE,
      type: "Opaque",
      data: {
        "admin-user": "admin",
        "admin-password": "{{ .adminPassword }}",
      },
    },
  };
}

function grafanaAdminPasswordDataFrom(): crd.ExternalSecretSpecDataFrom {
  return {
    sourceRef: {
      generatorRef: {
        apiVersion: PasswordApiVersion,
        kind: GeneratorKind.PASSWORD,
        name: GRAFANA_ADMIN_PASSWORD_GENERATOR_NAME,
      },
    },
  };
}
