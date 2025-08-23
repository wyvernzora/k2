import { BlockingGroup } from "./blocking-group";

export interface ClientGroupProps {
  /**
   * Name of the client group.
   * Can contain wildcards:
   *  - `*` sequence of any character
   *  - `[0-9]` character range
   * Can also be a single IP address or subnet in CIDR notation.
   */
  readonly name: string;
  /**
   * List of upstream resolvers to use for the group.
   * @default empty
   */
  readonly upstream?: string[];
  /**
   * Blocking groups to use for this client group.
   * @default empty
   */
  readonly blockingGroups?: BlockingGroup[];
}

/**
 * Represents a set of clients, def
 */
export class ClientGroup {
  readonly name: string;
  readonly upstream: string[] = [];
  readonly blockingGroups: BlockingGroup[] = [];

  constructor(props: ClientGroupProps) {
    this.name = props.name;
    this.upstream.push(...(props.upstream || []));
    this.blockingGroups.push(...(props.blockingGroups || []));
  }

  /**
   * Adds an upstream resolver to be used by this client group.
   * Ignores the resolver if it is already being used by this client group.
   */
  public addUpstream(address: string): ClientGroup {
    if (!this.upstream.some(val => val === address)) {
      this.upstream.push(address);
    }
    return this;
  }

  /**
   * Adds a {@link BlockingGroup} to be used by this client group.
   * If a blocking group of the same name is already being used, throws an error.
   */
  public useBlockingGroup(group: BlockingGroup): ClientGroup {
    if (this.blockingGroups.some(gp => gp.name === group.name)) {
      throw new Error(`BlockingGroup ${group.name} already used by ClientGroup ${this.name}`);
    }
    this.blockingGroups.push(group);
    return this;
  }
}
