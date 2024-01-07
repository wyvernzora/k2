import {Chart} from "cdk8s";
import {Construct} from "constructs";
import {GlauthConfig} from "./config";
import {GlauthCertificate} from "./certificate";
import {GlauthDeployment} from "./deployment";
import {Service, ServiceType} from "cdk8s-plus-27";
import {GlauthUsers} from "./users";


export class GlauthChart extends Chart {
    public readonly service: Service;

    constructor(scope: Construct, id: string) {
        super(scope, id);

        const config = new GlauthConfig(this, 'config', {
            domain: 'wyvernzora.io',
            ldapPort: 389,
            ldapsPort: 636,
        });
        const users = new GlauthUsers(this, 'users');
        const certificate = new GlauthCertificate(this, 'cert', {
            domain: 'wyvernzora.io',
        });
        const deployment = new GlauthDeployment(this, 'depl', {
            config: config,
            users: users,
            certificate: certificate,
        });
        this.service = deployment.exposeViaService({
            serviceType: ServiceType.LOAD_BALANCER,
        });
    }

}