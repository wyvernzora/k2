import { ContainerProps, Deployment, Protocol } from "cdk8s-plus-27";
import { Construct } from "constructs";

export class UnboundDeployment extends Deployment {

    constructor(scope: Construct, id: string) {
        super(scope, id, {
            replicas: 1,
        });
        this.addContainer(this.createUnboundContainer());
    }

    private createUnboundContainer(): ContainerProps {
        return {
            name: 'unbound',
            image: 'mvance/unbound:1.17.0',
            ports: [{
                name: 'dns-udp',
                number: 53,
                protocol: Protocol.UDP,
            }],
            securityContext: {
                ensureNonRoot: false,
            },
        }
    }

}
