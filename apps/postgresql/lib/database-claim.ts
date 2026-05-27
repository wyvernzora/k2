import type { Construct } from "constructs";

import {
  DatabaseClaim as DatabaseClaimResource,
  type DatabaseClaimProps as DatabaseClaimResourceProps,
  type DatabaseClaimSpec,
} from "../crds/cnpg.wyvernzora.io.js";

import { NEXUS_CLUSTER_NAME, NEXUS_CLUSTER_NAMESPACE } from "./nexus.js";

export { DatabaseClaimSpecDeletionPolicy } from "../crds/cnpg.wyvernzora.io.js";

export interface DatabaseClaimProps extends Omit<DatabaseClaimResourceProps, "spec"> {
  readonly spec: Omit<DatabaseClaimSpec, "clusterRef">;
}

export class DatabaseClaim extends DatabaseClaimResource {
  public constructor(scope: Construct, id: string, props: DatabaseClaimProps) {
    super(scope, id, {
      ...props,
      spec: {
        ...props.spec,
        clusterRef: {
          name: NEXUS_CLUSTER_NAME,
          namespace: NEXUS_CLUSTER_NAMESPACE,
        },
      },
    });
  }
}
