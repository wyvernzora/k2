import { Construct } from "constructs";
import { ISecret, Secret } from "cdk8s-plus-32";

import { DatabaseClaim, DatabaseClaimSpecDeletionPolicy, RoleClaim, RoleClaimSpecAccess } from "@k2/postgresql";

export class PaperlessDatabase extends Construct {
  public readonly credentials: ISecret;

  constructor(scope: Construct, id: string) {
    super(scope, id);

    new DatabaseClaim(this, "claim", {
      metadata: {
        name: "paperless",
      },
      spec: {
        databaseName: "paperless",
        deletionPolicy: DatabaseClaimSpecDeletionPolicy.RETAIN,
        schemas: ["public"],
      },
    });

    new RoleClaim(this, "role", {
      metadata: {
        name: "paperless",
      },
      spec: {
        access: RoleClaimSpecAccess.OWNER,
        databaseClaimRef: {
          name: "paperless",
        },
        roleName: "paperless",
      },
    });

    this.credentials = Secret.fromSecretName(this, "credentials", "paperless-credentials");
  }
}
