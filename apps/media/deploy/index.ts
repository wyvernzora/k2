import { App, K2Volume } from "@k2/cdk-lib";
import { Media } from "../components";
import { Size } from "cdk8s";

const app = new App();

new Media(app, "media", {
  qbitTorrent: {
    host: "dl.wyvernzora.io",
    volumes: {
      appdata: K2Volume.replicated({ size: Size.gibibytes(4) }),
    },
    downloads: {
      default: K2Volume.bulk({
        path: "/mnt/data/downloads",
      }),
      anime: K2Volume.bulk({
        path: "/mnt/data/media/anime",
      }),
    },
  },
  prowlarr: {
    url: "https://media.wyvernzora.io/prowlarr",
    volumes: {
      appdata: K2Volume.replicated({
        size: Size.gibibytes(4),
      }),
    },
  },
  sonarr: {
    url: "https://media.wyvernzora.io/sonarr",
    volumes: {
      appdata: K2Volume.replicated({
        size: Size.gibibytes(8),
      }),
      anime: K2Volume.bulk({
        path: "/mnt/data/media/anime/series",
      }),
    },
  },
  kavita: {
    url: "https://media.wyvernzora.io/kavita",
    volumes: {
      appdata: K2Volume.replicated({
        size: Size.gibibytes(1),
      }),
      library: K2Volume.bulk({
        path: "/mnt/data/media/manga/library",
      }),
    },
  },
});

app.synth();
