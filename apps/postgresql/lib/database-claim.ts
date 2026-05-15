import { Construct } from "constructs";

import {
  DatabaseClaim as DatabaseClaimResource,
  DatabaseClaimProps as DatabaseClaimResourceProps,
  DatabaseClaimSpec,
} from "../crds/cnpg.wyvernzora.io.js";

import { NEXUS_CLUSTER_NAME, NEXUS_CLUSTER_NAMESPACE } from "./nexus.js";

export interface DatabaseClaimProps extends Omit<DatabaseClaimResourceProps, "spec"> {
  readonly spec: Omit<DatabaseClaimSpec, "clusterRef">;
}

/**
 * DatabaseClaim against the K2 Nexus CNPG cluster. Hides the cluster's
 * name/namespace so consumer apps only describe the database they want.
 */
export class DatabaseClaim extends DatabaseClaimResource {
  constructor(scope: Construct, id: string, props: DatabaseClaimProps) {
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
