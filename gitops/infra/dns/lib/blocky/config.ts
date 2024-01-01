import { ConfigMap, ResourceProps } from "cdk8s-plus-27"
import { Construct } from "constructs"
import * as YAML from "yaml"

const DEFAULT_CLIENT_GROUP = "default";
const DEFAULT_BLOCKLIST_GROUP = "default";

export type BlockyConfigProps = ResourceProps & {
    readonly upstreams: string[]
    readonly blockLists?: string[]
    readonly allowLists?: string[]
}

export class BlockyConfig extends ConfigMap {

    constructor(scope: Construct, id: string, props: BlockyConfigProps) {
        super(scope, id, {
            ...props,
            metadata: {
                name: 'blocky-depl',
                ...props.metadata
            },
        });
        this.addData("config.yaml", YAML.stringify({
            upstream: {
                [DEFAULT_CLIENT_GROUP]: props.upstreams,
            },
            blocking: {
                blackLists: {
                    [DEFAULT_BLOCKLIST_GROUP]: props.blockLists || [],
                },
                clientGroupsBlock: {
                    [DEFAULT_CLIENT_GROUP]: [DEFAULT_BLOCKLIST_GROUP],
                },
            },
            ports: {
                dns: 53,
                http: 4000,
            },
        }));
    }

}
