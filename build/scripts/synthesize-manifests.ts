import { copyFile, mkdir, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { basename, join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

import { Chart } from "cdk8s";
import fg from "fast-glob";
import * as YAML from "yaml";

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
  const appOutdir = join(deployRoot, appName);
  const hasCrdManifest = await copyCrdManifest(appPath, appOutdir);
  await stripSynthesizedCrds(appPath, appOutdir, hasCrdManifest);

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

async function copyCrdManifest(appPath: string, outdir: string): Promise<boolean> {
  const src = join(appPath, "crds", "crds.k8s.yaml");
  try {
    await copyFile(src, join(outdir, "crds.k8s.yaml"));
    return true;
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
    return false;
  }
}

async function stripSynthesizedCrds(appPath: string, outdir: string, hasCrdManifest: boolean): Promise<void> {
  const appName = basename(appPath);
  const manifestPath = join(outdir, "app.k8s.yaml");
  let manifest: string;
  try {
    manifest = await readFile(manifestPath, "utf8");
  } catch (cause) {
    if ((cause as NodeJS.ErrnoException).code === "ENOENT") {
      return;
    }
    throw cause;
  }

  const documents = YAML.parseAllDocuments(manifest);
  const keptDocuments: YAML.Document[] = [];
  const crdNames: string[] = [];
  for (const document of documents) {
    if (document.get("kind") === "CustomResourceDefinition") {
      const name = document.getIn(["metadata", "name"]);
      crdNames.push(typeof name === "string" ? name : "(unnamed)");
      continue;
    }
    keptDocuments.push(document);
  }

  if (crdNames.length === 0) {
    return;
  }
  if (!hasCrdManifest) {
    throw new Error(
      `[${appName}] synthesized ${crdNames.length} CRD(s) into app.k8s.yaml, ` +
        `but apps/${appName}/crds/crds.k8s.yaml is missing. Commit upstream CRDs there instead.`,
    );
  }

  const rendered = keptDocuments.map(document => document.toString()).join("---\n");
  await writeFile(manifestPath, rendered.endsWith("\n") ? rendered : `${rendered}\n`);
  console.warn(`[${appName}] stripped ${crdNames.length} CRD(s) from app.k8s.yaml; using crds.k8s.yaml instead`);
}
