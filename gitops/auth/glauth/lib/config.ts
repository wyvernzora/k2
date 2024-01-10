import {ConfigMap} from "cdk8s-plus-27";
import {Construct} from "constructs";


export interface GlauthConfigProps {
    readonly ldapPort: number
    readonly domain: string
}

export class GlauthConfig extends ConfigMap {

    constructor(scope: Construct, id: string, props: GlauthConfigProps) {
        super(scope, id, { });
        this.addData("config.cfg", `
[ldap]
    enabled = true
    listen = "0.0.0.0:${props.ldapPort}"
[ldaps]
    enabled = false
[backend]
    datastore = "config"
    baseDN = "${props.domain.split('.').map((s) => `dc=${s}`).join(',')}"
[behaviors]
    IgnoreCapabilities = false
    LimitFailedBinds = true
    NumberOfFailedBinds = 3
    PeriodOfFailedBinds = 10
    BlockFailedBindsFor = 60
    PruneSourceTableEvery = 600
    PruneSourcesOlderThan = 600
        `);
    }

}
