import { App } from "@k2/cdk-lib";
import { BlockyChart } from "./lib";

const app = new App();
const appName = process.env.ARGOCD_APP_NAME || "blocky";
new BlockyChart(app, appName, {
  blockLists: [
    "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
  ],
});

app.synth();
