import { Size } from "cdk8s";
import { PlexChart } from "../components/plex/chart";
import { App, K2Volume } from "@k2/cdk-lib";

const app = new App();
new PlexChart(app, "plex", {
  host: "plex.wyvernzora.io",
  volumes: {
    config: K2Volume.replicated({
      size: Size.gibibytes(50),
    }),
    series: K2Volume.bulk({
      path: "/mnt/data/media/anime/series",
      readOnly: true,
    }),
    features: K2Volume.bulk({
      path: "/mnt/data/media/anime/features",
      readOnly: true,
    }),
    airing: K2Volume.bulk({
      path: "/mnt/data/media/anime/airing",
      readOnly: true,
    }),
  },
});
app.synth();
