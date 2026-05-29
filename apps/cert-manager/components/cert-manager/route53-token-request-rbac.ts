import { k8s } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { Namespace } from "@k2/cdk-lib";

import { CERT_MANAGER_CONTROLLER_SERVICE_ACCOUNT_NAME, ROUTE53_DNS01_TOKEN_REQUEST_ROLE_NAME } from "./constants.js";

export interface Route53TokenRequestRbacProps {
  readonly serviceAccountName: string;
}

export class Route53TokenRequestRbac extends Construct {
  public constructor(scope: Construct, id: string, props: Route53TokenRequestRbacProps) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- cdk8s-plus Role L2 does not expose resourceNames for serviceaccounts/token.
    new k8s.KubeRole(this, "role", {
      metadata: { name: ROUTE53_DNS01_TOKEN_REQUEST_ROLE_NAME },
      rules: [
        {
          apiGroups: [""],
          resources: ["serviceaccounts/token"],
          resourceNames: [props.serviceAccountName],
          verbs: ["create"],
        },
      ],
    });
    // eslint-disable-next-line k2/prefer-cdk8s-plus-l2 -- Keep the raw RoleBinding paired with the resourceNames-constrained Role above.
    new k8s.KubeRoleBinding(this, "role-binding", {
      metadata: { name: ROUTE53_DNS01_TOKEN_REQUEST_ROLE_NAME },
      roleRef: {
        apiGroup: "rbac.authorization.k8s.io",
        kind: "Role",
        name: ROUTE53_DNS01_TOKEN_REQUEST_ROLE_NAME,
      },
      subjects: [
        {
          kind: "ServiceAccount",
          name: CERT_MANAGER_CONTROLLER_SERVICE_ACCOUNT_NAME,
          namespace,
        },
      ],
    });
  }
}
