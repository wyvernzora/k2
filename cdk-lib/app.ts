import * as base from "cdk8s";
import { YamlOutputType } from "cdk8s";
import { dirname } from "path";
import { mkdir, writeFile } from "fs/promises";
import { Context, ContextClass } from "./context";

export class App extends base.App {
  constructor(...options: Array<AppOptionFunc>) {
    super({ yamlOutputType: YamlOutputType.FILE_PER_APP });
    options.forEach(opt => opt(this));
  }

  use<C extends ContextClass<any[]>>(Ctor: C, ...args: Parameters<C["with"]>): this;
  use(opt: AppOptionFunc): this;

  use(first: any, ...rest: any[]): this {
    if (Context.isContextClass(first)) {
      const opt = first.with(...rest);
      opt(this);
      return this;
    }
    if (isAppOptionFunc(first)) {
      first(this);
      return this;
    }
    throw new TypeError("App.use() expects an AppOptionFunc or a Context class");
  }

  async synthToFile(path: string): Promise<void> {
    const output = this.synthYaml();
    await mkdir(dirname(path), { recursive: true });
    await writeFile(path, output, "utf8");
  }
}

// Option that gets applied to the app
export type AppOptionFunc = (app: App) => void;

/** Type guards for runtime dispatch */
function isAppOptionFunc(x: unknown): x is AppOptionFunc {
  return typeof x === "function" && x.length >= 1; // (app) => void
}

export type AppResourceFunc = (app: App) => void;

export type ArgoCDResourceFunc = (chart: base.Chart) => void;

export function defineAppExports<
  T extends {
    createAppResources: AppResourceFunc;
    createArgoCdResources: ArgoCDResourceFunc;
    crds?: object;
  },
>(m: T): T {
  return m;
}
