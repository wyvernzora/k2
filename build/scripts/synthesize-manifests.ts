import { copyFile, mkdir, readdir, rm } from "node:fs/promises";
import { basename, join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

import { Chart } from "cdk8s";
import fg from "fast-glob";

import {
  ApexDomain,
  AppRoot,
  ClusterContext,
  HelmCharts,
  K2App,
  type K2AppDefinition,
  type K2AppInfo,
  Namespace,
  NfsContext,
  loadClusterConfig,
} from "@k2/cdk-lib";
import { makeDefaultArgoApplication } from "@k2/argocd";

const cluster = await loadClusterConfig();
const deployRoot = resolve("deploy");

await rm(deployRoot, { recursive: true, force: true });
await mkdir(deployRoot, { recursive: true });

const appPaths = (await fg("apps/*", { onlyDirectories: true })).sort();
const modules = await loadAppModules(appPaths);
// Argo bundle synthesizes to deploy/app.k8s.yaml — root of the deploy branch.
// Per-app manifests synthesize to deploy/<name>/app.k8s.yaml as siblings.
const argoApp = new K2App({ outdir: deployRoot }).use(ClusterContext, cluster);
const argoChart = new Chart(argoApp, "app");

for (const [appPath, mod] of modules) {
  const appName = basename(appPath);
  const appInfo = makeAppInfo(appName, appPath);

  const start = performance.now();
  const app = makeApp(appPath, join(deployRoot, appName));
  mod.createAppResources(app);
  app.synth();
  await copyCrdManifest(appPath, join(deployRoot, appName));

  makeDefaultArgoApplication(argoChart, appInfo);

  console.log(`[${appName}] synth ${Math.round(performance.now() - start)}ms`);
}

argoApp.synth();

async function loadAppModules(appPaths: string[]): Promise<Array<[string, K2AppDefinition]>> {
  const results = await Promise.allSettled(
    appPaths.map(async appPath => {
      const entry = resolve(appPath, "index.ts");
      try {
        const mod = (await import(pathToFileURL(entry).href)) as Partial<K2AppDefinition>;
        validateAppModule(appPath, mod);
        return [appPath, mod] as [string, K2AppDefinition];
      } catch (cause) {
        throw new Error(`Failed loading ${appPath}/index.ts`, { cause });
      }
    }),
  );

  const modules: Array<[string, K2AppDefinition]> = [];
  const errors: Error[] = [];
  for (const result of results) {
    if (result.status === "fulfilled") {
      modules.push(result.value);
    } else {
      errors.push(result.reason as Error);
    }
  }
  if (errors.length > 0) {
    const message = errors.map(error => `  - ${error.message}: ${(error.cause as Error)?.message ?? ""}`).join("\n");
    throw new Error(`Failed to load ${errors.length} app module(s):\n${message}`);
  }
  return modules;
}

function validateAppModule(appPath: string, mod: Partial<K2AppDefinition> | undefined): asserts mod is K2AppDefinition {
  if (typeof mod?.createAppResources !== "function") {
    throw new Error(`${appPath}/index.ts is missing named export: createAppResources`);
  }
}

function makeAppInfo(appName: string, appPath: string): K2AppInfo {
  return {
    name: appName,
    appPath,
    deployPath: join("deploy", appName),
    sourcePath: appName,
    destinationNamespace: appName,
  };
}

function makeApp(appPath: string, outdir: string): K2App {
  return new K2App({ outdir })
    .use(ClusterContext, cluster)
    .use(AppRoot, appPath)
    .use(HelmCharts, appPath)
    .use(Namespace, basename(appPath))
    .use(ApexDomain, cluster.apexDomain)
    .use(NfsContext, cluster.nfs.server, cluster.nfs.zone);
}

async function copyCrdManifest(appPath: string, outdir: string): Promise<void> {
  const src = join(appPath, "crds", "crds.k8s.yaml");
  try {
    await copyFile(src, join(outdir, "crds.k8s.yaml"));
  } catch (cause) {
    if ((cause as NodeJS.ErrnoException).code !== "ENOENT") {
      throw cause;
    }
    // No top-level crds.k8s.yaml. Warn if a crds/ directory exists anyway —
    // most likely the upstream-CRD copy step was skipped.
    try {
      await readdir(join(appPath, "crds"));
      console.warn(`[${basename(appPath)}] WARN: apps/${basename(appPath)}/crds/ exists but has no crds.k8s.yaml`);
    } catch {
      // No crds/ dir either — app has no CRDs, fine.
    }
  }
}
