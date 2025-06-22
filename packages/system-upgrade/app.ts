import { App } from "@k2/cdk-lib";
import { Chart, Include } from "cdk8s";

const app = new App();
new (class extends Chart {
  constructor() {
    super(app, "suc", {
      namespace: "k2-core",
    });
    new Include(this, "incl", {
      url: "https://github.com/rancher/system-upgrade-controller/releases/download/v0.15.2/system-upgrade-controller.yaml",
    });
  }
})();
app.synth();
