import { App, K2Volume } from "@k2/cdk-lib";
import { N8N } from "../constructs";
import { Size } from "cdk8s";

const app = new App();

new N8N(app, "n8n", {
  url: "https://n8n.wyvernzora.io/",
  volumes: {
    appdata: K2Volume.replicated({ size: Size.gibibytes(16) }),
  },
});

app.synth();
