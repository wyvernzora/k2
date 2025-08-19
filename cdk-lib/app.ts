import * as base from "cdk8s";
import { YamlOutputType } from "cdk8s";
import { Construct } from "constructs";

export class App extends base.App {
  constructor(...options: Array<AppOptionFunc>) {
    super({ yamlOutputType: YamlOutputType.FILE_PER_APP });
    options.forEach(opt => opt(this));
  }
}

// Option that gets applied to the app
export type AppOptionFunc = (app: App) => void;

export type AppResourceFunc = (app: App) => void;

export type ArgoCDResourceFunc = (chart: base.Chart) => void;

export function defineAppExports<
  T extends {
    createAppResources: AppResourceFunc;
    createArgoCdResources: ArgoCDResourceFunc;
    crds: object;
  },
>(m: T): T {
  return m;
}

/**
 * Abstract context value that can be passed down the CDK construct tree.
 */
export abstract class Context {
  protected abstract get ContextKey(): string;

  /**
   * @returns the instance of the context object from CDK construct context.
   */
  public static of<C extends { new (...args: any[]): Context }>(this: C, construct: Construct): InstanceType<C> {
    const key = (this.prototype as any).ContextKey;
    return construct.node.getContext(key) as InstanceType<C>;
  }

  /**
   * @returns an AppOptionFunc that attaches the context object to the CDK construct context.
   */
  public static with<C extends { new (...args: any[]): Context }>(
    this: C,
    ...args: ConstructorParameters<C>
  ): AppOptionFunc {
    return app => {
      const inst = new this(...args);
      app.node.setContext(inst.ContextKey, inst);
    };
  }
}
