import { Construct } from "constructs";

import { DatabaseClaim, DatabaseClaimSpecDeletionPolicy, RoleClaim, RoleClaimSpecAccess } from "@k2/postgresql";

const DATABASE_CLAIM_NAME = "forgejo";
const DATABASE_NAME = "forgejo";
const ROLE_NAME = "forgejo";
const DeletionPolicy = DatabaseClaimSpecDeletionPolicy;
const RoleAccess = RoleClaimSpecAccess;

export class ForgejoDatabase extends Construct {
  public readonly credentialsSecretName = `${DATABASE_CLAIM_NAME}-credentials`;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new DatabaseClaim(this, "database-claim", {
      metadata: { name: DATABASE_CLAIM_NAME },
      spec: {
        databaseName: DATABASE_NAME,
        deletionPolicy: DeletionPolicy.RETAIN,
        schemas: ["public"],
      },
    });

    new RoleClaim(this, "role-claim", {
      metadata: { name: DATABASE_CLAIM_NAME },
      spec: {
        access: RoleAccess.OWNER,
        databaseClaimRef: { name: DATABASE_CLAIM_NAME },
        roleName: ROLE_NAME,
      },
    });
  }
}
