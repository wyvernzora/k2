import { fileURLToPath } from "node:url";

import { Cron } from "cdk8s";
import { EnvValue, Secret, Volume } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { ScriptedCronJob } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";

import { DEFAULT_CERTIFICATE_SECRET_NAME } from "../../cert-manager/constants.js";
import { truenasHost, TRUENAS_PORT } from "../hosts.js";
import { CERT_SYNC_TRUENAS_LABELS } from "../labels.js";

const JOB_NAME = "cert-sync-truenas";
const CREDENTIAL_SECRET_NAME = "cert-sync-truenas";
const CREDENTIAL_SECRET_ID = "lfsuj4pkgrxtprzlcadbrgixze";
const SCRIPT_PATH = fileURLToPath(new URL("./scripts/sync.py", import.meta.url));

export class TrueNasCertSync extends Construct {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const credentials = new ManagedSecret(this, "credentials", {
      metadata: { name: CREDENTIAL_SECRET_NAME },
      secretId: CREDENTIAL_SECRET_ID,
      fields: {
        "api-key": "api-key",
      },
    });
    const certificate = Secret.fromSecretName(this, "certificate", DEFAULT_CERTIFICATE_SECRET_NAME);

    new ScriptedCronJob(this, "cron-job", {
      name: JOB_NAME,
      schedule: Cron.schedule({ minute: "17", hour: "*/6" }),
      script: {
        path: SCRIPT_PATH,
        filename: "sync.py",
      },
      labels: CERT_SYNC_TRUENAS_LABELS,
      env: {
        TRUENAS_HOST: EnvValue.fromValue(JSON.stringify(truenasHost(this))),
        TRUENAS_PORT: EnvValue.fromValue(String(TRUENAS_PORT)),
        TRUENAS_CERTIFICATE_NAME: EnvValue.fromValue("k2-default-certificate"),
      },
      mounts: [
        {
          volume: Volume.fromSecret(this, "certificate-volume", certificate, { name: "certificate" }),
          path: "/cert",
          readOnly: true,
        },
        {
          volume: Volume.fromSecret(this, "credentials-volume", credentials.secret, { name: "credentials" }),
          path: "/credentials",
          readOnly: true,
        },
      ],
    });
  }
}
