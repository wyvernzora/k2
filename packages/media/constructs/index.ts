import { Construct } from "constructs";
import { QBitTorrent, QBitTorrentProps } from "./qbittorrent";
import { Chart, ChartProps } from "cdk8s";
import { Prowlarr, ProwlarrProps } from "./prowlarr";
import { Sonarr, SonarrProps } from "./sonarr";
import { Kavita, KavitaProps } from "./kavita";

export interface MediaProps extends ChartProps {
  readonly qbitTorrent: QBitTorrentProps;
  readonly prowlarr: ProwlarrProps;
  readonly sonarr: SonarrProps;
  readonly kavita: KavitaProps;
}

export class Media extends Chart {
  readonly qbitTorrent: QBitTorrent;
  readonly prowlarr: Prowlarr;
  readonly sonarr: Sonarr;
  readonly kavita: Kavita;

  constructor(scope: Construct, id: string, props: MediaProps) {
    super(scope, id, { ...props });
    this.qbitTorrent = new QBitTorrent(this, "qbit", props.qbitTorrent);
    this.prowlarr = new Prowlarr(this, "prowlarr", props.prowlarr);
    this.sonarr = new Sonarr(this, "sonarr", props.sonarr);
    this.kavita = new Kavita(this, "kavita", props.kavita);
  }
}
