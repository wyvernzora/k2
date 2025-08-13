import { App } from "@k2/cdk-lib";
import { GlauthChart } from "../constructs";

const app = new App();
new GlauthChart(app, "glauth");

app.synth();
