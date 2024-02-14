import { App, YamlOutputType } from "cdk8s";
import { BlockyChart } from "./lib";

const app = new App({
  yamlOutputType: YamlOutputType.FILE_PER_APP,
});

const appName = process.env.ARGOCD_APP_NAME || "blocky";
new BlockyChart(app, appName, {
  blockLists: [
    "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
  ],
});

app.synth();
