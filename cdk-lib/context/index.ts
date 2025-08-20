import { Construct } from "constructs";
import { AppOptionFunc } from "..";

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

export * from "./apex-domain";
