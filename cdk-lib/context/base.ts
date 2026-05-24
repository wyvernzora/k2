import type { IConstruct } from "constructs";

export abstract class Context {
  public abstract readonly key: string;

  public static get<T extends Context>(scope: IConstruct, key: string): T {
    const value = scope.node.getContext(key);
    if (!(value instanceof Context)) {
      throw new Error(`Missing required context: ${key}`);
    }
    return value as T;
  }
}

export interface ContextClass<T extends Context, Args extends unknown[]> {
  readonly contextKey: string;
  new (...args: Args): T;
}

export type AppOption = (app: IConstruct) => void;

export function contextOption<T extends Context, Args extends unknown[]>(
  Ctor: ContextClass<T, Args>,
  ...args: Args
): AppOption {
  return app => {
    const context = new Ctor(...args);
    app.node.setContext(Ctor.contextKey, context);
  };
}
