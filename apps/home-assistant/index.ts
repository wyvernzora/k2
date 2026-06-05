import type { AppResourceFunc } from "@k2/cdk-lib";

import {
  endpoint,
  tcp,
  type BackendTarget,
  type PolicyEndpoint,
  type PrivateConnectionTarget,
} from "../cilium/lib/netpol/index.js";

import { HomeAssistant } from "./components/home-assistant/index.js";
import { Mosquitto } from "./components/mosquitto/index.js";
import { NetworkPolicy } from "./components/network-policy.js";
import { Zigbee2Mqtt } from "./components/zigbee2mqtt/index.js";
import {
  HOME_ASSISTANT_HTTP_PORT,
  HOME_ASSISTANT_LABELS,
  MOSQUITTO_LABELS,
  MOSQUITTO_MQTT_PORT,
  ZIGBEE2MQTT_HTTP_PORT,
  ZIGBEE2MQTT_LABELS,
} from "./constants.js";

const HOME_ASSISTANT_NAMESPACE = "home-assistant";

export const endpoints = {
  http(): BackendTarget {
    return { backend: homeAssistantEndpoint(), ports: [tcp(HOME_ASSISTANT_HTTP_PORT)] };
  },

  mosquittoMqtt(): PrivateConnectionTarget {
    return { to: mosquittoEndpoint(), ports: [tcp(MOSQUITTO_MQTT_PORT)] };
  },

  zigbee2mqttHttp(): BackendTarget {
    return { backend: zigbee2mqttEndpoint(), ports: [tcp(ZIGBEE2MQTT_HTTP_PORT)] };
  },
};

function homeAssistantEndpoint(): PolicyEndpoint {
  return endpoint(HOME_ASSISTANT_NAMESPACE, HOME_ASSISTANT_LABELS, "home-assistant");
}

function mosquittoEndpoint(): PolicyEndpoint {
  return endpoint(HOME_ASSISTANT_NAMESPACE, MOSQUITTO_LABELS, "mosquitto");
}

function zigbee2mqttEndpoint(): PolicyEndpoint {
  return endpoint(HOME_ASSISTANT_NAMESPACE, ZIGBEE2MQTT_LABELS, "zigbee2mqtt");
}

export const createAppResources: AppResourceFunc = app => {
  new Mosquitto(app, "mosquitto");
  new Zigbee2Mqtt(app, "zigbee2mqtt");
  new HomeAssistant(app, "home-assistant");
  new NetworkPolicy(app, "network-policy");
};
