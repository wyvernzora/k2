import { Construct } from "constructs";
import { AppOption } from "..";

const kContextClass = Symbol("@k2/cdk-lib:symbol:context-class");

/**
 * Abstract context value that can be passed down the CDK construct tree.
 */
export abstract class Context {
  protected abstract get ContextKey(): string;
  public static [kContextClass] = true;

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
  ): AppOption {
    return app => {
      const inst = new this(...args);
      app.node.setContext(inst.ContextKey, inst);
    };
  }

  static isContextClass(x: unknown): x is ContextClass<any[]> {
    return typeof x === "function" && x != null && (x as any)[kContextClass] === true;
  }
  static isContextInstance(x: unknown): x is Context {
    if (!x || typeof x !== "object") {
      return false;
    }
    const ctor = Object.getPrototypeOf(x)?.constructor;
    return typeof ctor === "function" && ctor[kContextClass] === true;
  }
}

// A constructor for a Context subclass that also has a static `with(...args)`
// returning an AppOptionFunc. The tuple A is the constructor args.
export type ContextClass<A extends any[] = any[]> = (abstract new (...args: A) => Context) & {
  with(...args: A): AppOption;
};

export * from "./apex-domain";
export * from "./app-root";
export * from "./helm-charts";
export * from "./namespace";
