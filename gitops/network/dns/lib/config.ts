import { ConfigMap } from "cdk8s-plus-27"
import { Construct } from "constructs"
import * as YAML from "yaml"

const DefaultClientGroup = "default"
const DefaultBlocklistGroup = "default"

export interface DnsConfigProps {
    readonly blockLists: string[]
}

export class DnsConfig extends ConfigMap {

    constructor(scope: Construct, id: string, props: DnsConfigProps) {
        super(scope, id, { });
        this.addBlockyConfig(props.blockLists);
    }

    private addBlockyConfig(blockLists: string[]): void {
        this.addData('blocky.yaml', YAML.stringify({
            log: {
                level: 'debug',
            },
            upstream: {
                [DefaultClientGroup]: [`1.1.1.1`],
            },
            blocking: {
                blackLists: {
                    [DefaultBlocklistGroup]: blockLists || [],
                },
                clientGroupsBlock: {
                    [DefaultClientGroup]: [DefaultBlocklistGroup],
                },
            },
            ports: {
                dns: 53,
                http: 4000,
            },
        }));
    }

}
