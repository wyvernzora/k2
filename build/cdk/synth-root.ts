import { readFile } from "node:fs/promises";

import { Chart } from "cdk8s";

import { ClusterContext, K2App, type K2AppInfo, loadClusterConfig } from "@k2/cdk-lib";
import { makeDefaultArgoApplication } from "@k2/argocd";

const appInfoPath = process.argv[2];
if (appInfoPath === undefined) {
  throw new Error("Usage: synth-root.ts <app-info-json>");
}

const apps = JSON.parse(await readFile(appInfoPath, "utf8")) as K2AppInfo[];
const cluster = await loadClusterConfig();
const app = new K2App().use(ClusterContext, cluster);
const chart = new Chart(app, "app");

for (const appInfo of apps) {
  makeDefaultArgoApplication(chart, appInfo);
}

process.stdout.write(app.synthYaml());
