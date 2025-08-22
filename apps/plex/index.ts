import { AppResourceFunc, ArgoCDResourceFunc, K2Volume } from "@k2/cdk-lib";
import { ContinuousDeployment } from "@k2/argocd";
import { PlexChart } from "./components/plex/chart";
import { Size } from "cdk8s";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
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
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "plex");
};
