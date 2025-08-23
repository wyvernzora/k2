import { Construct } from "constructs";
import { ServiceType } from "cdk8s-plus-28";

import { HelmV1 } from "@k2/cdk-lib";

export interface K8sGatewayProps {
  // Number of replicas to schedule.
  readonly replicas?: number;

  // Apex domain for exposed services.
  readonly apexDomain: string;

  // Namespace that the gateway is deployed to
  readonly namespace: string;
}

/**
 * Deploys the k8s_gateway helm chart with a ClusterIP service.
 */
export class K8sGateway extends HelmV1 {
  constructor(scope: Construct, name: string, props: K8sGatewayProps) {
    super(scope, name, {
      namespace: props.namespace,
      chart: "helm:https://k8s-gateway.github.io/k8s_gateway/k8s-gateway@3.2.6",
      values: {
        domain: props.apexDomain,
        replicaCount: props.replicas || 3,
        service: {
          type: ServiceType.CLUSTER_IP,
          useTcp: true,
        },
      },
    });
  }
}
