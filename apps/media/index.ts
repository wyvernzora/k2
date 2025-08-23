import { Size } from "cdk8s";

import { AppResourceFunc, ArgoCDResourceFunc, K2Volume } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";

import { Media } from "./components";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
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
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "media");
};
