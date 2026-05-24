import type { IConstruct } from "constructs";

import { Context } from "./base.js";

export class ApexDomain extends Context {
  public static readonly contextKey = "k2.apexDomain";
  public readonly key = ApexDomain.contextKey;

  public constructor(public readonly apexDomain: string) {
    super();
  }

  public subdomain(name: string): string {
    return `${name}.${this.apexDomain}`;
  }

  public static of(scope: IConstruct): ApexDomain {
    return Context.get(scope, ApexDomain.contextKey);
  }
}
