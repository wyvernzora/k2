import { Construct } from "constructs";

/**
 * Properties for the Application Context.
 */
export interface AppContextProps {
  /**
   * The root path of the application
   */
  readonly root: string;
  /**
   * The name of the application.
   */
  readonly name: string;
}

/**
 * Represents the context for an application in the K2 cluster.
 */
export class AppContext implements AppContextProps {
  private static readonly CONTEXT_KEY = "@k2/context/app";

  public readonly root: string;
  public readonly name: string;

  constructor(props: AppContextProps) {
    this.root = props.root;
    this.name = props.name;
  }

  /**
   * Sets the application context for a given Construct.
   * @param c - The Construct to set the context for.
   * @param val - The application context properties to set.
   */
  public static set(c: Construct, val: AppContextProps): void {
    c.node.setContext(this.CONTEXT_KEY, new AppContext(val));
  }

  /**
   * Retrieves the application context associated with a given Construct.
   * @param c - The Construct from which to retrieve the context.
   * @returns The AppContext associated with the Construct.
   */
  public static of(c: Construct): AppContext {
    return c.node.getContext(this.CONTEXT_KEY) as AppContext;
  }
}
