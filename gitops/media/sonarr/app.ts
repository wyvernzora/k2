import { App, Size, YamlOutputType } from "cdk8s";
import { SonarrChart } from "./lib/chart";

const app = new App({
  yamlOutputType: YamlOutputType.FILE_PER_APP,
});

new SonarrChart(app, "sonarr", {
  host: "sonarr.wyvernzora.io",
  volumes: {
    config: {
      kind: "replicated",
      size: Size.gibibytes(4),
    },
    anime: {
      kind: "nas",
      path: "/mnt/data/media/anime/series",
    },
  },
});

app.synth();
