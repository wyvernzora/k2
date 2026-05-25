import { BlockingGroup } from "./blocking-group.js";

export interface ClientGroupProps {
  readonly name: string;
  readonly upstream: string[];
  readonly blockingGroups?: BlockingGroup[];
}

export class ClientGroup {
  public readonly name: string;
  public readonly upstream: string[];
  public readonly blockingGroups: BlockingGroup[];

  public constructor(props: ClientGroupProps) {
    this.name = props.name;
    this.upstream = [...props.upstream];
    this.blockingGroups = [...(props.blockingGroups ?? [])];
  }
}
