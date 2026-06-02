import type { Construct } from "constructs";

import { K2Chart, Namespace } from "@k2/cdk-lib";
import {
  EndpointNetworkPolicy,
  NamespaceBoundaryPolicy,
  cidr,
  egress,
  endpoint,
  PrivateConnection,
  tcp,
} from "@k2/cilium";
import { AllowPomeriumToBackend } from "@k2/pomerium";

import { HOME_ASSISTANT_HTTP_PORT, HOME_ASSISTANT_LABELS } from "./home-assistant/labels.js";
import { MOSQUITTO_LABELS, MOSQUITTO_MQTT_PORT } from "./mosquitto/labels.js";
import { ZIGBEE2MQTT_HTTP_PORT, ZIGBEE2MQTT_LABELS } from "./zigbee2mqtt/labels.js";

const ZIGBEE_COORDINATOR_CIDR = "10.10.229.62/32";
const ZIGBEE_COORDINATOR_PORT = 6638;
const COMPONENT_LABEL = "app.kubernetes.io/component";
const HOME_ASSISTANT_COMPONENT = "app";
const ZIGBEE2MQTT_COMPONENT = "zigbee2mqtt";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const homeAssistant = endpoint(namespace, HOME_ASSISTANT_LABELS, "home-assistant");
    const mosquitto = endpoint(namespace, MOSQUITTO_LABELS, "mosquitto");
    const zigbee2mqtt = endpoint(namespace, ZIGBEE2MQTT_LABELS, "zigbee2mqtt");
    const sameNamespace = endpoint(namespace, {}, "home-assistant-namespace");
    const sameNamespaceExceptMqttClients = endpoint(namespace, {}, "home-assistant-namespace-except-mqtt-clients", [
      { key: COMPONENT_LABEL, operator: "NotIn", values: [HOME_ASSISTANT_COMPONENT, ZIGBEE2MQTT_COMPONENT] },
    ]);

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "home-assistant-http-ingress-deny", {
      endpoint: homeAssistant,
      ingressDeny: [{ from: { endpoint: sameNamespace }, ports: [tcp(HOME_ASSISTANT_HTTP_PORT)] }],
    });
    new EndpointNetworkPolicy(this, "zigbee2mqtt-http-ingress-deny", {
      endpoint: zigbee2mqtt,
      ingressDeny: [{ from: { endpoint: sameNamespace }, ports: [tcp(ZIGBEE2MQTT_HTTP_PORT)] }],
    });
    new EndpointNetworkPolicy(this, "mosquitto-ingress-deny", {
      endpoint: mosquitto,
      ingressDeny: [{ from: { endpoint: sameNamespaceExceptMqttClients }, ports: [tcp(MOSQUITTO_MQTT_PORT)] }],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-home-assistant", {
      backend: homeAssistant,
      ports: [tcp(HOME_ASSISTANT_HTTP_PORT)],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-zigbee2mqtt", {
      backend: zigbee2mqtt,
      ports: [tcp(ZIGBEE2MQTT_HTTP_PORT)],
    });
    new PrivateConnection(this, "home-assistant-to-mosquitto", {
      from: homeAssistant,
      to: mosquitto,
      ports: [tcp(MOSQUITTO_MQTT_PORT)],
    });
    new PrivateConnection(this, "zigbee2mqtt-to-mosquitto", {
      from: zigbee2mqtt,
      to: mosquitto,
      ports: [tcp(MOSQUITTO_MQTT_PORT)],
    });
    new EndpointNetworkPolicy(this, "home-assistant-egress", {
      endpoint: homeAssistant,
      egress: [...egress.toCidrs(cidr.rfc1918()), ...egress.toWorld(tcp(80), tcp(443))],
    });
    new EndpointNetworkPolicy(this, "zigbee2mqtt-egress", {
      endpoint: zigbee2mqtt,
      egress: [...egress.toCidrs([ZIGBEE_COORDINATOR_CIDR], tcp(ZIGBEE_COORDINATOR_PORT))],
    });
  }
}
