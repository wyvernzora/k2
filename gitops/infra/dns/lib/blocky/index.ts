import { Service, ServiceType } from "cdk8s-plus-27";
import { Construct } from "constructs";
import { BlockyConfig, BlockyConfigProps } from "./config";
import { BlockyDeployment } from "./deployment";
import { Chart, ChartProps } from "cdk8s";

export type BlockyAppProps = ChartProps & BlockyConfigProps;

export class BlockyApp extends Chart {
    public readonly service: Service;

    constructor(scope: Construct, id: string, props: BlockyAppProps) {
        super(scope, id);
        const config = new BlockyConfig(this, 'blocky-config', props);
        const deploy = new BlockyDeployment(this, 'blocky-depl', { config });
        this.service = deploy.exposeViaService({ serviceType: ServiceType.LOAD_BALANCER });
    }

}
