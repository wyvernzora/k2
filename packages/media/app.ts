import { App, K2Volume } from "@k2/cdk-lib";
import { Media } from "./lib";
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
});

app.synth();
