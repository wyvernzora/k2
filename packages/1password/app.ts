import { K2App } from "@k2/cdk-lib";
import { OnePasswordChart } from "./lib/chart";

const app = new K2App();
new OnePasswordChart(app, "1password", {
  namespace: "k2-core",
});
app.synth();
