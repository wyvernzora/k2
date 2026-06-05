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

import { endpoints } from "../index.js";

const ZIGBEE_COORDINATOR_CIDR = "10.10.229.62/32";
const ZIGBEE_COORDINATOR_PORT = 6638;
const COMPONENT_LABEL = "app.kubernetes.io/component";
const HOME_ASSISTANT_COMPONENT = "app";
const ZIGBEE2MQTT_COMPONENT = "zigbee2mqtt";

export class NetworkPolicy extends K2Chart {
  public constructor(scope: Construct, id: string) {
    super(scope, id);

    const namespace = Namespace.of(this).namespace;
    const homeAssistantHttp = endpoints.http();
    const mosquittoMqtt = endpoints.mosquittoMqtt();
    const zigbee2mqttHttp = endpoints.zigbee2mqttHttp();
    const sameNamespace = endpoint(namespace, {}, "home-assistant-namespace");
    const sameNamespaceExceptMqttClients = endpoint(namespace, {}, "home-assistant-namespace-except-mqtt-clients", [
      { key: COMPONENT_LABEL, operator: "NotIn", values: [HOME_ASSISTANT_COMPONENT, ZIGBEE2MQTT_COMPONENT] },
    ]);

    new NamespaceBoundaryPolicy(this, "namespace-boundary");
    new EndpointNetworkPolicy(this, "home-assistant-http-ingress-deny", {
      endpoint: homeAssistantHttp.backend,
      ingressDeny: [{ from: { endpoint: sameNamespace }, ports: homeAssistantHttp.ports }],
    });
    new EndpointNetworkPolicy(this, "zigbee2mqtt-http-ingress-deny", {
      endpoint: zigbee2mqttHttp.backend,
      ingressDeny: [{ from: { endpoint: sameNamespace }, ports: zigbee2mqttHttp.ports }],
    });
    new EndpointNetworkPolicy(this, "mosquitto-ingress-deny", {
      endpoint: mosquittoMqtt.to,
      ingressDeny: [{ from: { endpoint: sameNamespaceExceptMqttClients }, ports: mosquittoMqtt.ports }],
    });
    new AllowPomeriumToBackend(this, "pomerium-to-home-assistant", {
      ...homeAssistantHttp,
    });
    new AllowPomeriumToBackend(this, "pomerium-to-zigbee2mqtt", {
      ...zigbee2mqttHttp,
    });
    new PrivateConnection(this, "home-assistant-to-mosquitto", {
      from: homeAssistantHttp.backend,
      ...mosquittoMqtt,
    });
    new PrivateConnection(this, "zigbee2mqtt-to-mosquitto", {
      from: zigbee2mqttHttp.backend,
      ...mosquittoMqtt,
    });
    new EndpointNetworkPolicy(this, "home-assistant-egress", {
      endpoint: homeAssistantHttp.backend,
      egress: [...egress.toCidrs(cidr.rfc1918()), ...egress.toWorld(tcp(80), tcp(443))],
    });
    new EndpointNetworkPolicy(this, "zigbee2mqtt-egress", {
      endpoint: zigbee2mqttHttp.backend,
      egress: [...egress.toCidrs([ZIGBEE_COORDINATOR_CIDR], tcp(ZIGBEE_COORDINATOR_PORT))],
    });
  }
}
