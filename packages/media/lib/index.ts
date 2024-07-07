import { Construct } from "constructs";
import { QBitTorrent, QBitTorrentProps } from "./qbittorrent";
import { Chart, ChartProps } from "cdk8s";

export interface MediaProps extends ChartProps {
  readonly qbitTorrent: QBitTorrentProps;
}

export class Media extends Chart {
  readonly qbitTorrent: QBitTorrent;

  constructor(scope: Construct, id: string, props: MediaProps) {
    super(scope, id, { ...props });
    this.qbitTorrent = new QBitTorrent(this, "qbit", props.qbitTorrent);
  }
}
