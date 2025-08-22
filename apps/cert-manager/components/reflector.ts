import { HelmChartV1 } from "@k2/cdk-lib";
import { Construct } from "constructs";

export interface ReflectorProps {
  readonly namespace: string;
}

export class Reflector extends HelmChartV1 {
  constructor(scope: Construct, name: string, props: ReflectorProps) {
    super(scope, name, {
      namespace: props.namespace,
      chart: "helm:https://emberstack.github.io/helm-charts/reflector@9.1.26",
      values: {
        priorityClassName: "system-cluster-critical",
      },
    });
  }
}
