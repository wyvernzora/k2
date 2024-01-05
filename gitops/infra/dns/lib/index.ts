import { Chart } from "cdk8s";
import { Service, ServiceType } from "cdk8s-plus-27";
import { Construct } from "constructs";
import { DnsConfig } from "./config";
import { DnsDeployment } from "./deployment";


export interface DnsChartProps {
    readonly blockLists: string[]
}

export class DnsChart extends Chart {
    public readonly service: Service

    constructor(scope: Construct, id: string, props: DnsChartProps) {
        super(scope, id);
        const config = new DnsConfig(this, 'config', props);
        const deployment = new DnsDeployment(this, 'depl', { config, replicas: 2 });
        this.service = deployment.exposeViaService({
            serviceType: ServiceType.LOAD_BALANCER,
        });
        this.service.metadata.addAnnotation('metallb.universe.tf/loadBalancerIPs', '10.10.10.8');
    }

}
