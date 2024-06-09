import { Construct } from "constructs";
import { Chart } from "cdk8s";
import { Mosquitto, MosquittoProps } from "./mosquitto";
import { Zigbee2Mqtt, Zigbee2MqttProps } from "./zigbee2mqtt";
import { HomeAssistantDeploymentProps } from "./home-assistant/deployment";
import { Ingress, IngressBackend } from "cdk8s-plus-28";
import { HomeAssistant } from "./home-assistant";

export interface HomeAutomationProps {
  readonly namespace?: string;
  readonly hostname: string;
  readonly coordinator: string;
  readonly volumes?: {
    readonly mosquitto?: MosquittoProps["volumes"];
    readonly zigbee2mqtt?: Zigbee2MqttProps["volumes"];
    readonly homeAssistant?: HomeAssistantDeploymentProps["volumes"];
  };
}

export class HomeAutomation extends Chart {
  readonly mosquitto: Mosquitto;
  readonly zigbee2mqtt: Zigbee2Mqtt;
  readonly homeAssistant: HomeAssistant;
  readonly ingress: Ingress;

  constructor(scope: Construct, id: string, props: HomeAutomationProps) {
    super(scope, id, props);
    this.mosquitto = new Mosquitto(this, "mosquitto", {
      volumes: props.volumes?.mosquitto,
    });
    this.zigbee2mqtt = new Zigbee2Mqtt(this, "zigbee2mqtt", {
      mosquitto: this.mosquitto,
      coordinator: props.coordinator,
      volumes: props.volumes?.zigbee2mqtt,
    });
    this.homeAssistant = new HomeAssistant(this, "home-assistant", {
      mosquitto: this.mosquitto,
      volumes: props.volumes?.homeAssistant,
    });
    this.ingress = new Ingress(this, "ingress", {
      rules: [
        {
          host: props.hostname,
          backend: IngressBackend.fromService(this.homeAssistant.service),
        },
      ],
    });
  }
}
