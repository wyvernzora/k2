import { Construct } from "constructs";
import { QBitTorrent, QBitTorrentProps } from "./qbittorrent";
import { Chart, ChartProps } from "cdk8s";
import { Prowlarr, ProwlarrProps } from "./prowlarr";

export interface MediaProps extends ChartProps {
  readonly qbitTorrent: QBitTorrentProps;
  readonly prowlarr: ProwlarrProps;
}

export class Media extends Chart {
  readonly qbitTorrent: QBitTorrent;
  readonly prowlarr: Prowlarr;

  constructor(scope: Construct, id: string, props: MediaProps) {
    super(scope, id, { ...props });
    this.qbitTorrent = new QBitTorrent(this, "qbit", props.qbitTorrent);
    this.prowlarr = new Prowlarr(this, "prowlarr", props.prowlarr);
  }
}
