import { Construct } from "constructs";
import { Chart, ChartProps, Helm, HelmProps } from "cdk8s";
import { basename, dirname } from "path";

/**
 * Represents a reference to a Helm chart, including its repository and version.
 * Created from a reference string that looks like the following:
 *   helm!https://github.com/example/repo/chart-name?version=1.2.3
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
      throw new Error(`HelmChartRef must start with 'helm!' marker`);
    }
    const url = new URL(value.substring(5));
    this.repo = `${url.protocol}//${url.host}${dirname(url.pathname)}`;
    const [chart, version] = basename(url.pathname).split("@");
    this.chart = chart;
    this.version = version;
  }
}

export interface HelmChartProps extends ChartProps {
  readonly chart: HelmChartRef | string;
  readonly values: HelmProps["values"];
}

/**
 * HelmChart synthesizes a chart specified by HelmChartRef.
 */
export class HelmChart extends Chart {
  readonly helm: Helm;
  constructor(scope: Construct, name: string, props: HelmChartProps) {
    super(scope, name, props);
    const chartRef =
      typeof props.chart === "string"
        ? new HelmChartRef(props.chart)
        : props.chart;

    this.helm = new Helm(this, "helm", {
      namespace: this.namespace,
      releaseName: name,
      values: props.values,
      // Include CRDs so that K2 build process can handle them
      helmFlags: ["--include-crds"],
      ...chartRef,
    });
  }
}
