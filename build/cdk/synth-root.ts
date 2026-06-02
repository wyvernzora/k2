import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

import { Chart } from "cdk8s";

import { ClusterContext, K2App, type K2AppDefinition, type K2AppInfo, loadClusterConfig } from "@k2/cdk-lib";
import { makeDefaultArgoApplication } from "@k2/argocd";

const appInfoPath = process.argv[2];
if (appInfoPath === undefined) {
  throw new Error("Usage: synth-root.ts <app-info-json>");
}

const apps = await Promise.all((JSON.parse(await readFile(appInfoPath, "utf8")) as K2AppInfo[]).map(configureAppInfo));
const cluster = await loadClusterConfig();
const app = new K2App().use(ClusterContext, cluster);
const chart = new Chart(app, "app");

for (const appInfo of apps) {
  makeDefaultArgoApplication(chart, appInfo);
}

process.stdout.write(app.synthYaml());

async function configureAppInfo(appInfo: K2AppInfo): Promise<K2AppInfo> {
  const mod = (await import(pathToFileURL(resolve(appInfo.appPath, "index.ts")).href)) as Partial<K2AppDefinition>;
  if (mod.configureArgoApplication === undefined) {
    return appInfo;
  }
  if (typeof mod.configureArgoApplication !== "function") {
    throw new Error(`${appInfo.appPath}/index.ts exported configureArgoApplication, but it is not a function`);
  }
  return {
    ...appInfo,
    argoApplication: mod.configureArgoApplication(appInfo),
  };
}
