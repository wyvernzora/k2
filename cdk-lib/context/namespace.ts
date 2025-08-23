import { Context } from "../context.js";

export class Namespace extends Context {
  get ContextKey() {
    return "@k2/cdk-lib:ns";
  }

  constructor(public readonly namespace: string) {
    super();
  }
}
