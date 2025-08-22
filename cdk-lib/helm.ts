import { Construct } from "constructs";
import * as base from "cdk8s";
import { basename, dirname } from "path";

/**
 * Represents a reference to a Helm chart, including its repository and version.
 * Created from a reference string that looks like the following:
 *   helm:https://github.com/example/repo/chart-name@1.2.3
 * The rationale for introducing this is to make it easier for dependency management
 * tools such as Renovate detect and update Helm chart references in what are normally
 * considered source code files.
 */
export class HelmChartRef {
  readonly repo: string;
  readonly chart: string;
  readonly version?: string;

  constructor(value: string) {
    if (!value.startsWith("helm:")) {
      throw new Error(`HelmChartRef must start with 'helm:' marker`);
    }
    const url = new URL(value.substring(5));
    this.repo = `${url.protocol}//${url.host}${dirname(url.pathname)}`;
    const [chart, version] = basename(url.pathname).split("@");
    this.chart = chart;
    this.version = version;
  }
}

export interface HelmProps {
  /**
   * Reference to a Helm chart.
   * Must be a {@link HelmChartRef} or a string that can be made into one.
   */
  readonly chart: HelmChartRef | string;
  /**
   * Namespace to deploy the Helm chart to.
   * @default undefined
   */
  readonly namespace?: string;
  /**
   * Values to supply to the helm chart, if any.
   * @default undefined
   */
  readonly values?: base.HelmProps["values"];
}

/**
 * Extended version of the Helm construct that uses the special Helm chart reference
 * string as input. See {@link HelmChartRef}
 */
export class Helm extends base.Helm {
  constructor(scope: Construct, name: string, props: HelmProps) {
    const chart = typeof props.chart === "string" ? new HelmChartRef(props.chart) : props.chart;

    super(scope, name, {
      namespace: props.namespace,
      releaseName: name,
      values: props.values,
      helmFlags: ["--skip-crds"],
      ...chart,
    });
    this.removeCustomResourceDefinitions();
  }

  removeCustomResourceDefinitions(): void {
    for (const child of this.node.children) {
      if (base.ApiObject.isApiObject(child) && child.kind === "CustomResourceDefinition") {
        this.node.tryRemoveChild(child.node.id);
      }
    }
  }
}

export interface HelmChartProps extends base.ChartProps, HelmProps {}

/**
 * HelmChart synthesizes a Chart specified by HelmChartRef.
 */
export class HelmChart extends base.Chart {
  readonly helm: Helm;
  constructor(scope: Construct, name: string, props: HelmChartProps) {
    super(scope, name, props);
    this.helm = new Helm(this, name, {
      namespace: this.namespace,
      ...props,
    });
  }
}
