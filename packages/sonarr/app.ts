import { App, Size, YamlOutputType } from "cdk8s";
import { SonarrChart } from "./lib/chart";
import { K2Volume } from "@k2/cdk-lib";

const app = new App({
  yamlOutputType: YamlOutputType.FILE_PER_APP,
});

new SonarrChart(app, "sonarr", {
  host: "sonarr.wyvernzora.io",
  volumes: {
    config: K2Volume.replicated({
      size: Size.gibibytes(4),
    }),
    anime: K2Volume.bulk({
      path: "/mnt/data/media/anime/series",
    }),
  },
});

app.synth();
