import type { K2App } from "./app.js";

export interface K2AppInfo {
  readonly name: string;
  readonly appPath: string;
  readonly deployPath: string;
  readonly sourcePath: string;
  readonly destinationNamespace: string;
}

export type AppResourceFunc = (app: K2App) => void;

/**
 * An app module exports `createAppResources` — synth derives everything else
 * (namespace from dir name, Argo Application from cluster config + app info).
 * Apps don't author Argo Applications; synth does. If an app ever needs to
 * customize sync policy / project / multi-source, add the hook then.
 */
export interface K2AppDefinition {
  readonly createAppResources: AppResourceFunc;
}
