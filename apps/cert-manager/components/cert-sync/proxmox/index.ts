import { fileURLToPath } from "node:url";

import { Cron } from "cdk8s";
import { EnvValue, Secret, Volume } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { ScriptedCronJob } from "@k2/cdk-lib";
import { ManagedSecret } from "@k2/external-secrets";

import { DEFAULT_CERTIFICATE_SECRET_NAME } from "../../cert-manager/constants.js";
import { PROXMOX_PORT, proxmoxHosts } from "../hosts.js";
import { CERT_SYNC_PROXMOX_LABELS } from "../labels.js";

const JOB_NAME = "cert-sync-proxmox";
const CREDENTIAL_SECRET_NAME = "cert-sync-proxmox";
const CREDENTIAL_SECRET_ID = "iyvvxsn6rsrxawh6o3wyapzijm";
const SCRIPT_PATH = fileURLToPath(new URL("./scripts/sync.py", import.meta.url));

export class ProxmoxCertSync extends Construct {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const credentials = new ManagedSecret(this, "credentials", {
      metadata: { name: CREDENTIAL_SECRET_NAME },
      secretId: CREDENTIAL_SECRET_ID,
      fields: {
        "api-token-id": "api-token-id",
        "api-token-secret": "api-token-secret",
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
      labels: CERT_SYNC_PROXMOX_LABELS,
      env: {
        PROXMOX_HOSTS: EnvValue.fromValue(JSON.stringify(proxmoxHosts(this))),
        PROXMOX_PORT: EnvValue.fromValue(String(PROXMOX_PORT)),
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
