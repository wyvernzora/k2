import { App } from "@k2/cdk-lib";
import { GlauthChart } from "../components/glauth";

const app = new App();
new GlauthChart(app, "glauth");

app.synth();
