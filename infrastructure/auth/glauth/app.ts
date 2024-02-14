import { App, YamlOutputType } from "cdk8s";
import { GlauthChart } from "./lib";

const app = new App({
  yamlOutputType: YamlOutputType.FILE_PER_APP,
});

new GlauthChart(app, "glauth");

app.synth();
