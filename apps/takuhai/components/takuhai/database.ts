import { Construct } from "constructs";

import { DatabaseClaim, DatabaseClaimSpecDeletionPolicy, RoleClaim, RoleClaimSpecAccess } from "@k2/postgresql";

const DATABASE_CLAIM_NAME = "takuhai";
const DeletionPolicy = DatabaseClaimSpecDeletionPolicy;
const RoleAccess = RoleClaimSpecAccess;

export class TakuhaiDatabase extends Construct {
  public readonly credentialsSecretName = `${DATABASE_CLAIM_NAME}-credentials`;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new DatabaseClaim(this, "database-claim", {
      metadata: { name: DATABASE_CLAIM_NAME },
      spec: {
        databaseName: DATABASE_CLAIM_NAME,
        deletionPolicy: DeletionPolicy.RETAIN,
        schemas: ["public"],
      },
    });

    new RoleClaim(this, "role-claim", {
      metadata: { name: DATABASE_CLAIM_NAME },
      spec: {
        access: RoleAccess.OWNER,
        databaseClaimRef: { name: DATABASE_CLAIM_NAME },
        roleName: DATABASE_CLAIM_NAME,
      },
    });
  }
}
