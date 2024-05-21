import { Chart } from "cdk8s";
import { Construct } from "constructs";
import { Blocky, BlockyProps } from "./blocky";
import { K8sGateway, K8sGatewayProps } from "./gateway";
import { Service } from "cdk8s-plus-28";

export {
  BlockingGroup,
  BlockingGroupProps,
  ClientGroup,
  ClientGroupProps,
  CustomDns,
  CustomDnsProps,
} from "./blocky";
export { K8sGatewayProps } from "./gateway";

export interface DnsProps extends BlockyProps, K8sGatewayProps {
  readonly namespace?: string;
}

export class Dns extends Chart {
  readonly blocky: Blocky;
  readonly gateway: K8sGateway;
  readonly service: Service;

  constructor(scope: Construct, id: string, props: DnsProps) {
    super(scope, id, { namespace: props.namespace });
    this.gateway = new K8sGateway(this, "k8s-gateway", { ...props });
    this.blocky = new Blocky(this, "blocky", { ...props });
    this.service = this.blocky.service;
  }
}
