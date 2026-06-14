import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import { DatabaseClaim, DatabaseClaimSpecDeletionPolicy, RoleClaim, RoleClaimSpecAccess } from "@k2/postgresql";

import { POMERIUM_DATABASE_CLAIM_NAME, POMERIUM_DATABASE_NAME, POMERIUM_DATABASE_ROLE_NAME } from "../../constants.js";

import { PomeriumStorageSecret } from "./storage-secret.js";

const DeletionPolicy = DatabaseClaimSpecDeletionPolicy;
const RoleAccess = RoleClaimSpecAccess;

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

    new PomeriumStorageSecret(this, "storage-secret", {
      namespace: Namespace.of(this).namespace,
    });
  }
}
