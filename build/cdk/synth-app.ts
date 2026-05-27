import { basename, resolve } from "node:path";
import { pathToFileURL } from "node:url";

import {
  ApexDomain,
  AppRoot,
  ClusterContext,
  HelmCharts,
  K2App,
  type K2AppDefinition,
  Namespace,
  NfsContext,
  loadClusterConfig,
} from "@k2/cdk-lib";

const appPath = process.argv[2];
if (appPath === undefined) {
  throw new Error("Usage: synth-app.ts <app-path>");
}

const cluster = await loadClusterConfig();
const mod = (await import(pathToFileURL(resolve(appPath, "index.ts")).href)) as Partial<K2AppDefinition>;
if (typeof mod.createAppResources !== "function") {
  throw new Error(`${appPath}/index.ts is missing named export: createAppResources`);
}

const app = new K2App()
  .use(ClusterContext, cluster)
  .use(AppRoot, appPath)
  .use(HelmCharts, appPath)
  .use(Namespace, basename(appPath))
  .use(ApexDomain, cluster.apexDomain)
  .use(NfsContext, cluster.nfs.server, cluster.nfs.zone);

mod.createAppResources(app);
process.stdout.write(app.synthYaml());
