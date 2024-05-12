import { Construct } from "constructs";
import { Chart, ChartProps, Helm } from "cdk8s";

const TOLERATE_CONTROL_PLANE = {
  tolerations: [
    {
      key: "CriticalAddonsOnly",
      operator: "Exists",
    },
    {
      key: "node-role.kubernetes.io/control-plane",
      operator: "Exists",
      effect: "NoSchedule",
    },
    {
      key: "node-role.kubernetes.io/master",
      operator: "Exists",
      effect: "NoSchedule",
    },
  ],
};

export interface OnePasswordChartProps extends ChartProps {}

export class OnePasswordChart extends Chart {
  readonly helm: Construct;

  constructor(scope: Construct, name: string, props: OnePasswordChartProps) {
    super(scope, name, props);

    this.helm = new Helm(this, "helm", {
      chart: "1password/connect",
      version: "1.15.0",
      namespace: props.namespace,
      releaseName: name,
      values: {
        connect: { ...TOLERATE_CONTROL_PLANE },
        operator: { create: true, ...TOLERATE_CONTROL_PLANE },
      },
    });
  }
}
