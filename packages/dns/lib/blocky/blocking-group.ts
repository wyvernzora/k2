export interface BlockingGroupProps {
  /**
   * Name of the blocking group.
   */
  readonly name: string;
  /**
   * List of blacklists.
   * Each item can be a URL pointing to a list, or an inline list.
   * @default {[]}
   */
  readonly blacklists?: string[];
  /**
   * List of whitelists.
   * Each item can be a URL pointing to a list, or an inline list.
   * @default {[]}
   */
  readonly whitelists?: string[];
}

/**
 * Represents a set of blocking configuration, consisting of sets of
 * blacklists and whitelists.
 */
export class BlockingGroup {
  readonly name: string;
  readonly blacklists: string[] = [];
  readonly whitelists: string[] = [];

  constructor(props: BlockingGroupProps) {
    this.name = props.name;
    this.blacklists.push(...(props.blacklists || []));
    this.whitelists.push(...(props.whitelists || []));
  }

  /**
   * Add one or more blacklists to this {@link BlockingGroup}
   * @param list First list
   * @param more Additional lists
   */
  public addBlacklist(list: string, ...more: string[]): void {
    this.blacklists.push(list, ...more);
  }

  /**
   * Add one or more whitelists to this {@link BlockingGroup}
   * @param list First list
   * @param more Additional lists
   */
  public addWhitelist(list: string, ...more: string[]): void {
    this.whitelists.push(list, ...more);
  }
}
