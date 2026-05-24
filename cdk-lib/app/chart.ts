import { Chart, type ChartProps } from "cdk8s";
import type { Construct } from "constructs";

import { AppRoot } from "../context/app-root.js";
import { Namespace } from "../context/namespace.js";

export class K2Chart extends Chart {
  public constructor(scope: Construct, id: string, props: ChartProps = {}) {
    const namespace = props.namespace ?? Namespace.of(scope).namespace;
    const appName = AppRoot.of(scope).appName;
    super(scope, id, {
      ...props,
      namespace,
      labels: {
        "k2.wyvernzora.io/app": appName,
        "k2.wyvernzora.io/component": id,
        ...props.labels,
      },
    });
  }
}
