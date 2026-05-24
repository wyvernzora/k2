import { readFileSync } from "node:fs";
import { join } from "node:path";

import { Helm, type HelmProps } from "cdk8s";
import type { Construct, IConstruct } from "constructs";
import { parse } from "yaml";

import { Context } from "./base.js";
import { Namespace } from "./namespace.js";

interface ChartYaml {
  readonly dependencies?: HelmDependency[];
}

interface HelmDependency {
  readonly name: string;
  readonly alias?: string;
  readonly repository?: string;
  readonly version?: string;
}

export class HelmCharts extends Context {
  public static readonly contextKey = "k2.helmCharts";
  public readonly key = HelmCharts.contextKey;
  private readonly dependencies = new Map<string, HelmDependency>();

  public constructor(appPath: string) {
    super();
    const chartFile = join(appPath, "Chart.yaml");
    try {
      const chart = parse(readFileSync(chartFile, "utf8")) as ChartYaml;
      for (const dependency of chart.dependencies ?? []) {
        this.dependencies.set(dependency.alias ?? dependency.name, dependency);
      }
    } catch (cause) {
      if ((cause as NodeJS.ErrnoException).code !== "ENOENT") {
        throw cause;
      }
    }
  }

  public asProps(alias: string, values?: HelmProps["values"]): HelmProps {
    const dependency = this.dependencies.get(alias);
    if (dependency === undefined) {
      throw new Error(`No Helm dependency named ${alias}`);
    }

    if (dependency.repository?.startsWith("oci://") === true) {
      return {
        chart: `${dependency.repository}/${dependency.name}`,
        version: dependency.version,
        values,
      };
    }

    return {
      chart: dependency.name,
      repo: dependency.repository,
      version: dependency.version,
      values,
    };
  }

  public asChart(scope: Construct, id: string, alias = id, values?: HelmProps["values"]): Helm {
    return new Helm(scope, id, {
      namespace: Namespace.of(scope).namespace,
      releaseName: id,
      ...this.asProps(alias, values),
    });
  }

  public static of(scope: IConstruct): HelmCharts {
    return Context.get(scope, HelmCharts.contextKey);
  }
}
