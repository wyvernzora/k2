import { Collector } from "./collector";
import { RootApplication } from "./root-manifest";

(async function () {
  console.log("Generating deployment artifacts...");
  const collector = new Collector();
  const app = new RootApplication(collector);
  app.synth();
  await collector.copyManifests();
})().catch((err) => {
  console.error(err);
  process.exit(1);
});
