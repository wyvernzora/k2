import { fileURLToPath } from "node:url";

import { Cron } from "cdk8s";
import { EnvFieldPaths, EnvValue } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { K2Chart, ScriptedCronJob, ScriptedJob, type ScriptedJobRbacRule } from "@k2/cdk-lib";
import { DatabaseClaim, DatabaseClaimSpecDeletionPolicy, RoleClaim, RoleClaimSpecAccess } from "@k2/postgresql";

import {
  POMERIUM_DATABASE_CLAIM_NAME,
  POMERIUM_DATABASE_NAME,
  POMERIUM_DATABASE_ROLE_NAME,
  POMERIUM_DATABASE_SECRET_NAME,
} from "../constants.js";

const DeletionPolicy = DatabaseClaimSpecDeletionPolicy;
const RoleAccess = RoleClaimSpecAccess;
const SECRET_SYNC_JOB_NAME = "database-secret-sync";
const SECRET_SYNC_CRON_NAME = "database-secret-sync-cron";
const SECRET_SYNC_SCRIPT_PATH = fileURLToPath(new URL("./scripts/sync-database-secret.py", import.meta.url));

export class PomeriumDatabase extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new DatabaseClaim(this, "database-claim", {
      metadata: { name: POMERIUM_DATABASE_CLAIM_NAME },
      spec: {
        databaseName: POMERIUM_DATABASE_NAME,
        deletionPolicy: DeletionPolicy.RETAIN,
        schemas: ["public"],
      },
    });

    new RoleClaim(this, "role-claim", {
      metadata: { name: POMERIUM_DATABASE_CLAIM_NAME },
      spec: {
        access: RoleAccess.OWNER,
        databaseClaimRef: { name: POMERIUM_DATABASE_CLAIM_NAME },
        roleName: POMERIUM_DATABASE_ROLE_NAME,
      },
    });

    createDatabaseSecretSync(this);
  }
}

function createDatabaseSecretSync(scope: Construct): void {
  const props = {
    script: {
      path: SECRET_SYNC_SCRIPT_PATH,
      filename: "sync-database-secret.py",
    },
    env: secretSyncEnv(),
    labels: {
      "app.kubernetes.io/component": "database-secret-sync",
      "app.kubernetes.io/name": "pomerium",
    },
    rbacRules: [secretSyncRbacRule()],
  };

  new ScriptedJob(scope, "database-secret-sync-job", {
    name: SECRET_SYNC_JOB_NAME,
    ...props,
  });
  new ScriptedCronJob(scope, "database-secret-sync-cron", {
    name: SECRET_SYNC_CRON_NAME,
    schedule: Cron.schedule({ minute: "*/15" }),
    ...props,
  });
}

function secretSyncEnv(): Record<string, EnvValue> {
  return {
    POD_NAMESPACE: EnvValue.fromFieldRef(EnvFieldPaths.POD_NAMESPACE),
    POMERIUM_DATABASE_SECRET: EnvValue.fromValue(POMERIUM_DATABASE_SECRET_NAME),
  };
}

function secretSyncRbacRule(): ScriptedJobRbacRule {
  return {
    apiGroups: [""],
    resources: ["secrets"],
    resourceNames: [POMERIUM_DATABASE_SECRET_NAME],
    verbs: ["get", "patch", "update"],
  };
}
