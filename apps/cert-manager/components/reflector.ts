import { HelmChart } from "@k2/cdk-lib";
import { Construct } from "constructs";

export interface ReflectorProps {
  readonly namespace: string;
}

export class Reflector extends HelmChart {
  constructor(scope: Construct, name: string, props: ReflectorProps) {
    super(scope, name, {
      namespace: props.namespace,
      chart: "helm:https://emberstack.github.io/helm-charts/reflector@9.1.25",
      values: {
        priorityClassName: "system-cluster-critical",
      },
    });
  }
}
