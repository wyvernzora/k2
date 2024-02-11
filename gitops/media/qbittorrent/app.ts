import { App, Size, YamlOutputType } from "cdk8s";
import { QbitTorrentChart } from "./lib/chart";

const app = new App({
  yamlOutputType: YamlOutputType.FILE_PER_APP,
});

new QbitTorrentChart(app, "qbittorrent", {
  host: "dl.wyvernzora.io",
  volumes: {
    config: {
      kind: "replicated",
      size: Size.gibibytes(4),
    },
    downloads: {
      anime: {
        kind: "nas",
        path: "/mnt/data/media/anime/downloads",
      },
      airing: {
        kind: "nas",
        path: "/mnt/data/media/anime/airing",
      },
    },
  },
});

app.synth();
