import { Construct } from "constructs";
import { Chart, ChartProps } from "cdk8s";

import { QBitTorrent, QBitTorrentProps } from "./qbittorrent/index.js";
import { Prowlarr, ProwlarrProps } from "./prowlarr/index.js";
import { Kura, KuraProps } from "./kura/index.js";
import { Kavita, KavitaProps } from "./kavita/index.js";

export interface MediaProps extends ChartProps {
  readonly qbitTorrent: QBitTorrentProps;
  readonly prowlarr: ProwlarrProps;
  readonly kura: KuraProps;
  readonly kavita: KavitaProps;
}

export class Media extends Chart {
  readonly qbitTorrent: QBitTorrent;
  readonly prowlarr: Prowlarr;
  readonly kura: Kura;
  readonly kavita: Kavita;

  constructor(scope: Construct, id: string, props: MediaProps) {
    super(scope, id, { ...props });
    this.qbitTorrent = new QBitTorrent(this, "qbit", props.qbitTorrent);
    this.prowlarr = new Prowlarr(this, "prowlarr", props.prowlarr);
    this.kura = new Kura(this, "kura", props.kura);
    this.kavita = new Kavita(this, "kavita", props.kavita);
  }
}
