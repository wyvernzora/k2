import { Construct } from "constructs";
import { Chart } from "cdk8s";
import { Service } from "cdk8s-plus-27";
import { UnboundDeployment } from "./deployment";


export class UnboundApp extends Chart {
    public readonly service: Service;

    constructor(scope: Construct, id: string) {
        super(scope, id);
        const deploy = new UnboundDeployment(this, 'unbound-depl');
        this.service = deploy.exposeViaService();
    }
}
