import { AppOptionFunc } from "@k2/cdk-lib";
import { Context } from ".";
import { Helm, HelmProps as HelmPropsBase } from "cdk8s";
import * as findUp from "find-up";
import { join } from "node:path";
import { readFileSync } from "node:fs";
import * as yaml from "js-yaml";
import { AppRootContext } from "./app-root";
import { Construct } from "constructs";

export type HelmPropsV2 = Omit<HelmPropsBase, "chart" | "repo" | "version">;

export class HelmChartsContext extends Context {
  get ContextKey() {
    return "@k2/cdk-lib:helm-charts";
  }

  private readonly _charts: Partial<Record<string, ChartDependency[]>>;

  constructor(dependencies: ChartDependency[]) {
    super();
    this._charts = {
      // Allow charts to be referred by their full name (repo + chart)
      ...Object.groupBy(dependencies, c => join(c.repository, c.name)),
      // ...also allow them to be referred by just chart name, unless there is a collision
      ...Object.groupBy(dependencies, c => c.name),
      // ...also alias, where present, to avoid collisions
      ...Object.groupBy(
        dependencies.filter(c => !!c.alias),
        c => c.alias!,
      ),
    };
  }

  public chart(name: string) {
    const ref = this.findDependency(name);
    return class extends Helm {
      constructor(scope: Construct, id: string, props: HelmPropsV2) {
        super(scope, id, {
          chart: ref.name,
          repo: ref.repository,
          version: ref.version,
          releaseName: id,
          ...props,
        });
      }
    };
  }

  private findDependency(name: string): ChartDependency {
    const refs = this._charts[name];
    if (!refs || refs.length === 0) {
      throw new Error(`Chart not in dependencies: ${name}`);
    }
    if (refs.length > 1) {
      throw new Error(`Conflicting charts for name: ${name}; use full name or set unique aliases`);
    }
    return refs[0];
  }

  public static with(): AppOptionFunc {
    return app => {
      const { appRoot } = AppRootContext.of(app);
      const dependencies = getDependencyCharts(appRoot);
      const instance = new HelmChartsContext(dependencies);
      app.node.setContext(instance.ContextKey, instance);
    };
  }
}

export interface ChartDependency {
  name: string;
  version?: string;
  repository: string;
  alias?: string;
}

function getDependencyCharts(root: string): Array<ChartDependency> {
  const chartYamlPath = findUp.sync(["Chart.yaml"], { cwd: root });
  if (!chartYamlPath) {
    return []; // No Chart.yaml is equivalent to having no Helm dependencies
  }
  const chartData = yaml.load(readFileSync(chartYamlPath, "utf-8")) as {
    readonly dependencies?: Array<ChartDependency>;
  };
  return chartData.dependencies ?? [];
}
