import { isMainThread, parentPort, workerData } from "worker_threads";
import fg from "fast-glob";
import fs from "fs";
import path from "path";
import yaml from "js-yaml";
import { App, ApexDomainContext } from "@k2/cdk-lib";
import { Chart, ApiObject } from "cdk8s";
import * as OnePassword from "@k2/1password";
import * as ArgoCD from "@k2/argocd";

type WorkerData = { type: "app"; appPath: string } | { type: "argo" };

async function main() {
  // Discover all app dirs under ./apps
  const appPaths: string[] = await fg("apps/*", { onlyDirectories: true });
  const jobs: Promise<void>[] = [];

  // Spawn one worker per app
  for (const appPath of appPaths) {
    jobs.push(spawnWorker({ type: "app", appPath }));
  }

  // Spawn the Argo manifest worker
  jobs.push(spawnWorker({ type: "argo" }));

  await Promise.all(jobs);
  console.log("✅ All synth tasks complete");
}

async function synthAppManifest(appPath: string) {
  const appName = path.basename(appPath);
  const outputPath = path.resolve("deploy", appName, "app.k8s.yaml");

  // 1) Try createAppResources export in apps/$APP/index.ts
  const indexTs = path.resolve(appPath, "index.ts");
  if (fs.existsSync(indexTs)) {
    const mod = require(indexTs);
    if (typeof mod.createAppResources === "function") {
      console.log(`🚀 [V3] Synthesizing ${appName} CDK`);
      const app = new App(OnePassword.withDefaultVault(), ApexDomainContext.with("wyvernzora.io"));
      mod.createAppResources(app);
      await app.synthToFile(outputPath);
      copyCrdManifest(appPath);
      console.log(`✅ [V3] Synthesized ${appName} CDK`);
      return;
    }
  }

  // 2) If apps/$APP/deploy/index.ts exists, just run it
  const deployIndexTs = path.resolve(appPath, "deploy", "index.ts");
  if (fs.existsSync(deployIndexTs)) {
    console.log(`🚀 [V2] Synthesizing ${appName} CDK`);
    require(deployIndexTs);
    fs.mkdirSync(path.dirname(outputPath), { recursive: true });
    fs.renameSync("dist/app.k8s.yaml", outputPath);
    copyCrdManifest(appPath);
    console.log(`✅ [V2] Synthesized ${appName} CDK`);
    return;
  }

  console.log(`${appName} has no exported CDK synthesis methods`);
}

async function copyCrdManifest(appPath: string) {
  const appName = path.basename(appPath);
  const sourcePath = path.resolve(appPath, "crds/crds.k8s.yaml");
  const outputPath = path.resolve("deploy", appName, "crds.k8s.yaml");

  if (fs.existsSync(sourcePath)) {
    fs.copyFileSync(sourcePath, outputPath);
  }
}

async function synthArgoManifest() {
  const app = new App(
    OnePassword.withDefaultVault(),
    ArgoCD.withDefaultArgoCdOptions(),
    ApexDomainContext.with("wyvernzora.io"),
  );
  const chart = new Chart(app, "argocd");

  // For each app, either call its createArgoCdResources or attach its YAML
  const appPaths: string[] = await fg("apps/*", { onlyDirectories: true });
  for (const appPath of appPaths) {
    const appName = path.basename(appPath);

    // Try an exported function first
    const indexTs = path.resolve(appPath, "index.ts");
    if (fs.existsSync(indexTs)) {
      const mod = require(indexTs);
      if (typeof mod.createArgoCdResources === "function") {
        console.log(`🔄 [V3] Synthesizing ${appName} ArgoCD`);
        mod.createArgoCdResources(chart);
        continue;
      }
    }

    // Fallback to raw YAML file if present
    const yamlFile = path.resolve(appPath, "argocd.k8s.yaml");
    if (fs.existsSync(yamlFile)) {
      console.log(`🔄 [V2] Synthesizing ${appName} ArgoCD`);
      const docs = yaml.loadAll(await fs.promises.readFile(yamlFile, "utf8"));
      for (const doc of docs) {
        const name = (doc as any).metadata?.name;
        if (!name) {
          throw new Error(`Invalid ArgoCD manifest; no resource name: ${yamlFile}`);
        }
        new ApiObject(chart, name, doc as any);
      }
    }
  }

  // Synthesize and write to ./deploy/app.k8s.yaml
  await app.synthToFile("deploy/app.k8s.yaml");

  console.log(`✅ Synthesized ArgoCD manifest`);
}

async function spawnWorker(data: WorkerData): Promise<void> {
  /*
  return new Promise((resolve, reject) => {
    const w = new Worker(__filename, {
      workerData: data,
      execArgv: [
        // preserve node flags to enable TypeScript loading
        ...process.execArgv,
      ],
    });
    w.once("message", () => resolve());
    w.once("error", err => reject(err));
    w.once("exit", code => {
      if (code !== 0) {
        reject(new Error(`Worker for ${data.type} exited with code ${code}`));
      }
    });
  });
  */
  if (data.type === "app") {
    await synthAppManifest(data.appPath);
  } else {
    await synthArgoManifest();
  }
}

if (isMainThread) {
  main().catch(err => {
    console.error(err);
    process.exit(1);
  });
} else {
  // Worker thread entrypoint
  const data = workerData as WorkerData;
  (async () => {
    if (data.type === "app") {
      await synthAppManifest(data.appPath);
    } else {
      await synthArgoManifest();
    }
    parentPort!.postMessage("done");
  })().catch(err => {
    console.error(err);
    process.exit(1);
  });
}
