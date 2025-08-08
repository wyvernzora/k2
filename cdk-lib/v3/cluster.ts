import * as cdk from "cdk8s";
import { AppContext, ClusterConstants, ClusterContext, HelmContext } from "./context";
import { globSync } from "fast-glob";
import path from "node:path";

export interface K2ClusterProps {
  readonly rootPath: string;
  readonly clusterConstants: ClusterConstants;
}

export class K2Cluster extends cdk.App {
  private readonly root: string;
  private readonly context: ClusterContext;

  constructor(props: K2ClusterProps) {
    super();
    this.root = props.rootPath;
    this.context = new ClusterContext(props.rootPath, props.clusterConstants);
    this.node.setContext(ClusterContext.CONTEXT_KEY, this.context);
  }

  discoverApplications(): K2Cluster {
    const directories = globSync(`${this.root}/apps/*/`, { onlyDirectories: true });
    for (const dir of directories) {
      console.log(`Found application chart ${path.basename(dir)}`);
      const { createDeploymentChart } = require(dir);
      createDeploymentChart(this, path.basename(dir));
    }
    return this;
  }
}

export interface K2AppProps extends cdk.ChartProps {}

export class K2App extends cdk.Chart {
  constructor(cluster: K2Cluster, name: string, props?: K2AppProps) {
    super(cluster, name, props);
    const ctx = ClusterContext.of(cluster);
    AppContext.set(this, {
      name,
      root: ctx.getApplicationRoot(name),
    });
  }
}

export type K2AppFactoryFunc = (cluster: K2Cluster, name: string, props: K2AppProps) => K2App;

export const K2AppFactories = {
  helm(fn: (app: K2App, helm: HelmContext) => void): K2AppFactoryFunc {
    return (cluster, name, props) => {
      const app = new K2App(cluster, name, props);
      const helm = HelmContext.of(app);
      fn(app, helm);
      return app;
    };
  },
};
