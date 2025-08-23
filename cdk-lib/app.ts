/* eslint-disable @typescript-eslint/no-explicit-any */
import { dirname } from "path";
import { mkdir, writeFile } from "fs/promises";

import * as base from "cdk8s";
import { YamlOutputType } from "cdk8s";

import { Context, ContextClass } from "./context.js";

export class App extends base.App {
  constructor(...options: Array<AppOption>) {
    super({ yamlOutputType: YamlOutputType.FILE_PER_APP });
    options.forEach(opt => opt(this));
  }

  use<C extends ContextClass<any[]>>(Ctor: C, ...args: Parameters<C["with"]>): this;
  use(opt: AppOption): this;

  use(first: any, ...rest: any[]): this {
    if (Context.isContextClass(first)) {
      const opt = first.with(...rest);
      opt(this);
      return this;
    }
    if (isAppOption(first)) {
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

export type AppOption = (app: App) => void;

function isAppOption(x: unknown): x is AppOption {
  return typeof x === "function" && x.length >= 1; // (app) => void
}

export type AppResourceFunc = (app: App) => void;

export type ArgoCDResourceFunc = (chart: base.Chart) => void;
