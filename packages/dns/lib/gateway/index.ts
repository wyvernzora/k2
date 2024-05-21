import { Construct } from "constructs";
import { ServiceType } from "cdk8s-plus-28";
import { Helm, NodeAffinity, Toleration } from "@k2/cdk-lib";

export interface K8sGatewayProps {
  /**
   * Namespace to deploy the k8s_gateway chart into
   */
  readonly namespace?: string;
  /**
   * Number of replicas to schedule.
   */
  readonly replicas?: number;
  /**
   * Apex domain for exposed services.
   */
  readonly apexDomain: string;
}

/**
 * Deploys the k8s_gateway helm chart with a ClusterIP service.
 */
export class K8sGateway extends Helm {
  constructor(scope: Construct, name: string, props: K8sGatewayProps) {
    super(scope, name, {
      namespace: props.namespace,
      chart: "helm:https://ori-edge.github.io/k8s_gateway/k8s-gateway@2.4.0",
      values: {
        domain: props.apexDomain,
        replicaCount: props.replicas || 3,
        service: {
          type: ServiceType.CLUSTER_IP,
        },
        affinity: NodeAffinity.PREFER_CONTROL_PLANE,
        tolerations: [
          ...Toleration.ALLOW_CONTROL_PLANE,
          ...Toleration.ALLOW_CRITICAL_ADDONS_ONLY,
        ],
      },
    });
  }
}
