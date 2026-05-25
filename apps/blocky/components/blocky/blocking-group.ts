export interface BlockingGroupProps {
  readonly name: string;
  readonly blacklists?: string[];
  readonly whitelists?: string[];
}

export class BlockingGroup {
  public readonly name: string;
  public readonly blacklists: string[] = [];
  public readonly whitelists: string[] = [];

  public constructor(props: BlockingGroupProps) {
    this.name = props.name;
    this.blacklists.push(...(props.blacklists ?? []));
    this.whitelists.push(...(props.whitelists ?? []));
  }
}
