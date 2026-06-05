export const HOME_ASSISTANT_HTTP_PORT = 8123;
export const HOME_ASSISTANT_SERVICE_NAME = "home-assistant";

export const HOME_ASSISTANT_LABELS = {
  "app.kubernetes.io/name": "home-assistant",
  "app.kubernetes.io/component": "app",
};

export const MOSQUITTO_MQTT_PORT = 1883;
export const MOSQUITTO_SERVICE_NAME = "mosquitto";

export const MOSQUITTO_LABELS = {
  "app.kubernetes.io/name": "home-assistant",
  "app.kubernetes.io/component": "mosquitto",
};

export const ZIGBEE2MQTT_HTTP_PORT = 8080;
export const ZIGBEE2MQTT_SERVICE_NAME = "zigbee2mqtt";

export const ZIGBEE2MQTT_LABELS = {
  "app.kubernetes.io/name": "home-assistant",
  "app.kubernetes.io/component": "zigbee2mqtt",
};
