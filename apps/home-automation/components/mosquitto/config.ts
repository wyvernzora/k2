import { Construct } from "constructs";
import dedent from "dedent-js";

import { ConfigMap } from "@k2/cdk-lib";

export interface MosquittoConfigProps {
  readonly mqttPort?: number;
}
type Props = MosquittoConfigProps;

export class MosquittoConfig extends ConfigMap {
  constructor(scope: Construct, id: string, props: Props) {
    super(scope, id, {});
    this.addData("mosquitto.conf", this.renderMosquittoConf(props));
  }

  private renderMosquittoConf(props: Props) {
    return dedent`
      listener ${props.mqttPort || 1883}
      allow_anonymous true
      persistence true
      persistence_location /mosquitto/data/
      log_dest stdout
    `;
  }
}
