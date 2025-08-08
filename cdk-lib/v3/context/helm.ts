import { Helm, HelmProps } from "cdk8s";
import { Construct } from "constructs";
import { readFileSync } from "node:fs";
import * as yaml from "js-yaml";
import { join } from "node:path";
import * as findUp from "find-up";
import _ from "lodash";
import { AppContext } from "./app";

export interface HelmChartRef {
  readonly name: string;
  readonly version?: string;
  readonly repository: string;
}

export class HelmContext {
  private static readonly CONTEXT_KEY = "@k2/helm-context";
  private readonly _charts: Record<string, HelmChartRef[]>;

  private constructor(charts: Array<HelmChartRef>) {
    this._charts = {
      // Allow all charts to be referenced by their full name
      ..._.groupBy(charts, c => join(c.repository, c.name)),

      // Also allow charts to be referenced by short name (without repo URL)
      ..._.groupBy(charts, c => c.name),
    };
  }

  public chart(name: string) {
    const chartRefs = this._charts[name];
    if (!chartRefs || chartRefs.length === 0) {
      throw new Error(`Chart not in dependencies: ${name}`);
    }
    if (chartRefs.length > 1) {
      throw new Error(`Conflicting shortname for chart ${name}; use full name with repo URL`);
    }
    return class extends Helm {
      constructor(scope: Construct, id: string, props: Omit<HelmProps, "chart" | "repo" | "version">) {
        const chart = chartRefs[0];
        super(scope, id, {
          chart: chart.name,
          repo: chart.repository,
          version: chart.version,
          releaseName: id,
          ...props,
        });
      }
    };
  }

  public static of(c: Construct): HelmContext {
    const cached = c.node.tryGetContext(this.CONTEXT_KEY);
    if (cached) {
      return cached as HelmContext;
    }
    const appCtx = AppContext.of(c);
    const context = new HelmContext(getDependencyCharts(appCtx.root));
    c.node.setContext(this.CONTEXT_KEY, context);
    return context;
  }
}

// Retrieves a map of dependency charts and their versions
// Used by application context to construct Helm chart constructs
function getDependencyCharts(approot: string): Array<HelmChartRef> {
  const chartYamlPath = findUp.sync(["Chart.yaml"], { cwd: approot });
  if (!chartYamlPath) {
    return []; // No Chart.yaml is equivalent to no Helm dependencies
  }
  const chartData = yaml.load(readFileSync(chartYamlPath, "utf-8")) as {
    readonly dependencies: Array<HelmChartRef>;
  };
  return chartData.dependencies;
}
