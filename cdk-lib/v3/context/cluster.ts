import { Construct } from "constructs";
import path from "node:path";

export interface ClusterConstants {
  /**
   * Apex domain of cluster resources.
   * @example dev.example.com
   */
  readonly domain: string;
  /**
   * Cluster-global AWS configuration.
   */
  readonly aws: AWS;
}

interface AWS {
  /**
   * AWS region to use by default.
   */
  readonly region: string;
}

export class ClusterContext implements ClusterConstants {
  public static readonly CONTEXT_KEY = "@k2/cluster-context";

  readonly root: string;
  readonly domain;
  readonly aws;

  /**
   * Retrieves the cluster context for a given Construct.
   * @param c - The Construct from which to retrieve the context.
   * @returns The ClusterContext associated with the Construct.
   */
  public static of(c: Construct) {
    return c.node.getContext(this.CONTEXT_KEY) as ClusterContext;
  }

  /**
   * Constructs an instance of the ClusterContext.
   * @param root - The root path of the cluster context.
   * @param values - The cluster constants containing domain and AWS config.
   */
  constructor(root: string, values: ClusterConstants) {
    this.root = root;
    this.domain = values.domain;
    this.aws = values.aws;
  }

  /**
   * Retrieves the application root path for a given application name.
   * @param name - The name of the application.
   * @returns The full path of the application within the cluster context.
   */
  public getApplicationRoot(name: string): string {
    return path.join(this.root, "apps", name);
  }
}
