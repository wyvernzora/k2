import type { Construct } from "constructs";
import { ApiResource, Role, ServiceAccount } from "cdk8s-plus-32";

import { K2Chart, Namespace } from "@k2/cdk-lib";

import { DEFAULT_CERTIFICATE_REPLICATION_SERVICE_ACCOUNT_NAME } from "../cert-manager/constants.js";

import { DefaultCertificateClusterExternalSecret } from "./cluster-external-secret.js";
import { DefaultCertificateReplicationStore } from "./replication-store.js";

export class DefaultCertificateReplication extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const serviceAccount = new ServiceAccount(this, "service-account", {
      metadata: { name: DEFAULT_CERTIFICATE_REPLICATION_SERVICE_ACCOUNT_NAME },
      automountToken: false,
    });

    const role = new Role(this, "role", {
      metadata: { name: DEFAULT_CERTIFICATE_REPLICATION_SERVICE_ACCOUNT_NAME },
      rules: [
        {
          verbs: ["get", "list", "watch"],
          resources: [ApiResource.SECRETS],
        },
        {
          verbs: ["create"],
          resources: [ApiResource.SELF_SUBJECT_RULES_REVIEWS],
        },
      ],
    });
    role.bind(serviceAccount);

    new DefaultCertificateReplicationStore(this, "store", {
      serviceAccountName: serviceAccount.name,
      sourceNamespace: namespace,
    });
    new DefaultCertificateClusterExternalSecret(this, "replication");
  }
}
