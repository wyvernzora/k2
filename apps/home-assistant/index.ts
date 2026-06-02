import type { AppResourceFunc } from "@k2/cdk-lib";

import { HomeAssistant } from "./components/home-assistant/index.js";
import { Mosquitto } from "./components/mosquitto/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { Zigbee2Mqtt } from "./components/zigbee2mqtt/index.js";

export const createAppResources: AppResourceFunc = app => {
  new Mosquitto(app, "mosquitto");
  new Zigbee2Mqtt(app, "zigbee2mqtt");
  new HomeAssistant(app, "home-assistant");
  new NetworkPolicy(app, "network-policy");
};
