import { App, Chart } from "cdk8s";
import { Collector } from "./lib/collector";
import findRoot from "find-root";

const app = new App();
const chart = new Chart(app, "k2-root");
const collector = new Collector(chart, "collector", {
  root: findRoot(__dirname),
});
collector.collect();
collector.printWaves();
app.synth();
