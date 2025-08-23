import os from "os";
import fg from "fast-glob";
import fs from "fs";
import path from "path";
import pLimit from "p-limit";

import { App, ApexDomain, AppRoot, HelmCharts } from "@k2/cdk-lib";
import { Chart } from "cdk8s";
import * as OnePassword from "@k2/1password";
import * as ArgoCD from "@k2/argocd";

type Task = { type: "app"; appPath: string } | { type: "argo" };

async function main() {
  // 1) Discover all apps and build task list
  const appDirs = await fg("apps/*", { onlyDirectories: true });
  const tasks: Task[] = appDirs.map(appPath => ({ type: "app", appPath }));
  tasks.push({ type: "argo" });

  // 2) Concurrency limit (env MAX_CONCURRENCY or # of CPUs)
  const maxConcurrency = Number(process.env.MAX_CONCURRENCY) || os.cpus().length;
  const limit = pLimit(maxConcurrency);

  console.log(`⏳ Synthesizing manifests with concurrency=${maxConcurrency}`);

  // 3) Fire off all tasks through p-limit
  await Promise.all(
    tasks.map(task =>
      limit(async () => {
        if (task.type === "app") {
          await synthAppManifest(task.appPath);
        } else {
          await synthArgoManifest();
        }
      }),
    ),
  );

  console.log("✅ All synth tasks complete");
}

async function synthAppManifest(appPath: string) {
  const appName = path.basename(appPath);
  const outFile = path.resolve("deploy", appName, "app.k8s.yaml");

  console.log(`🚀 Synthesizing ${appName} CDK`);
  const mod = require(path.resolve(appPath, "index.ts"));
  if (typeof mod.createAppResources !== "function") {
    throw new Error(`[V3] ${appName}: missing createAppResources export`);
  }

  await new App()
    .use(AppRoot, appPath)
    .use(HelmCharts)
    .use(OnePassword.withDefaultVault())
    .use(ApexDomain, "wyvernzora.io")
    .use(app => mod.createAppResources(app))
    .synthToFile(outFile);

  copyCrdManifest(appPath);
  console.log(`✅ Synthesized ${appName} CDK`);
}

async function synthArgoManifest() {
  console.log("🔄 Synthesizing ArgoCD manifest");
  const app = new App()
    .use(OnePassword.withDefaultVault())
    .use(ArgoCD.withDefaultArgoCdOptions())
    .use(ApexDomain, "wyvernzora.io");
  const chart = new Chart(app, "argocd");

  const appDirs = await fg("apps/*", { onlyDirectories: true });
  for (const appPath of appDirs) {
    const appName = path.basename(appPath);
    const mod = require(path.resolve(appPath, "index.ts"));
    if (typeof mod.createArgoCdResources !== "function") {
      throw new Error(`[V3] ${appName}: missing createArgoCdResources export`);
    }
    console.log(`🔄 Synthesizing ${appName} ArgoCD`);
    mod.createArgoCdResources(chart);
  }

  await app.synthToFile("deploy/app.k8s.yaml");
  console.log(`✅ Synthesized ArgoCD manifest`);
}

function copyCrdManifest(appPath: string) {
  const appName = path.basename(appPath);
  const src = path.resolve(appPath, "crds/crds.k8s.yaml");
  const dst = path.resolve("deploy", appName, "crds.k8s.yaml");
  if (fs.existsSync(src)) {
    fs.copyFileSync(src, dst);
  }
}

// Entrypoint
main().catch(err => {
  console.error(err);
  process.exit(1);
});
