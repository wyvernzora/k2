import { Construct } from "constructs";

import { DatabaseClaim, DatabaseClaimSpecDeletionPolicy, RoleClaim, RoleClaimSpecSchemasAccess } from "@k2/postgresql";

const DATABASE_CLAIM_NAME = "kura";
const DATABASE_NAME = "kura";
const RELEASES_SCHEMA_NAME = "releases";
const ROLE_CLAIM_NAME = "release-indexer";
const ROLE_NAME = "release_indexer";
const DeletionPolicy = DatabaseClaimSpecDeletionPolicy;
const SchemaAccess = RoleClaimSpecSchemasAccess;

export class ReleaseIndexerDatabase extends Construct {
  public readonly credentialsSecretName = `${ROLE_CLAIM_NAME}-credentials`;

  public constructor(scope: Construct, id: string) {
    super(scope, id);

    new DatabaseClaim(this, "database-claim", {
      metadata: { name: DATABASE_CLAIM_NAME },
      spec: {
        databaseName: DATABASE_NAME,
        deletionPolicy: DeletionPolicy.RETAIN,
        schemas: [RELEASES_SCHEMA_NAME],
      },
    });

    new RoleClaim(this, "role-claim", {
      metadata: { name: ROLE_CLAIM_NAME },
      spec: {
        databaseClaimRef: { name: DATABASE_CLAIM_NAME },
        roleName: ROLE_NAME,
        schemas: [{ name: RELEASES_SCHEMA_NAME, access: SchemaAccess.OWNER }],
      },
    });
  }
}
