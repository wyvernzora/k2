import { Size } from "cdk8s";
import { QBitTorrentChart } from "./lib/chart";
import { App, K2Volume } from "@k2/cdk-lib";

const app = new App();

new QBitTorrentChart(app, "qbittorrent", {
  host: "dl.wyvernzora.io",
  volumes: {
    config: K2Volume.replicated({
      size: Size.gibibytes(4),
    }),
    default: K2Volume.bulk({
      path: "/mnt/data/downloads",
    }),
    anime: K2Volume.bulk({
      path: "/mnt/data/media/anime/downloads",
    }),
    airing: K2Volume.bulk({
      path: "/mnt/data/media/anime/airing",
    }),
  },
});

app.synth();
