import { Application } from "@k2/argocd";
import { K2App } from "@k2/cdk-lib";
import { Collector, Artifacts } from "./collector";
import { Chart } from "cdk8s";

export class RootApplication extends K2App {
  private readonly chart: Chart;

  constructor(collector: Collector) {
    super({
      outdir: "deploy",
    });
    this.chart = new Chart(this, "root", {
      namespace: "k2-core",
    });
    collector.waves.forEach((wave, index) => {
      wave
        .filter((i) => i.hasManifests)
        .forEach((app) => {
          this.createApplication(index, app);
        });
    });
  }

  private createApplication(wave: number, app: Artifacts) {
    new Application(this.chart, `${app.name}`, {
      metadata: {
        name: app.name,
        annotations: {
          "argocd.argoproj.io/sync-wave": `${wave}`,
        },
      },
      spec: {
        project: "default",
        source: {
          repoUrl: "https://github.com/wyvernzora/k2",
          path: app.name,
          targetRevision: "deploy",
        },
        destination: {
          server: "https://kubernetes.default.svc",
        },
        syncPolicy: {
          syncOptions: [
            "CreateNamespace=true",
            "ServerSideApply=true",
            "ApplyOutOfSyncOnly=true",
          ],
          automated: {
            prune: true,
            selfHeal: true,
          },
          retry: {
            limit: 10,
            backoff: {
              duration: "30s",
              maxDuration: "10m",
              factor: 2,
            },
          },
        },
      },
    });
  }
}
