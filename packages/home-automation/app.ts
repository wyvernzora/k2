import { App, K2Volume } from "@k2/cdk-lib";
import { HomeAutomation } from "./lib";
import { Size } from "cdk8s";

const app = new App();
new HomeAutomation(app, "home-automation", {
  namespace: "home-automation",
  hostname: "ha.wyvernzora.io",
  coordinator: "tcp://10.10.229.62:6638",
  volumes: {
    mosquitto: {
      data: K2Volume.replicated({
        size: Size.gibibytes(1),
      }),
    },
    zigbee2mqtt: {
      data: K2Volume.replicated({
        size: Size.gibibytes(1),
      }),
    },
    homeAssistant: {
      config: K2Volume.replicated({
        size: Size.gibibytes(1),
      }),
    },
  },
});
app.synth();
