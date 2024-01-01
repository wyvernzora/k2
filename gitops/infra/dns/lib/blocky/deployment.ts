import { ContainerProps, Deployment, Protocol, ResourceProps, Volume } from "cdk8s-plus-27";
import { Construct } from "constructs";
import { BlockyConfig } from "./config";

export type BlockyDeploymentProps = ResourceProps & {
    readonly config: BlockyConfig
}

export class BlockyDeployment extends Deployment {

    constructor(scope: Construct, id: string, props: BlockyDeploymentProps) {
        super(scope, id, {
            ...props,
            metadata: {
                name: 'blocky-depl',
                ...props.metadata
            },
        })
        const configVolume = this.createConfigVolume(props.config);
        const container = this.createBlockyContainer(configVolume);
        this.addContainer(container);
    }

    private createConfigVolume(config: BlockyConfig): Volume {
        return Volume.fromConfigMap(this, 'blocky-config', config, {

        });
    }

    private createBlockyContainer(configVolume: Volume): ContainerProps {
        return {
            name: 'blocky',
            image: 'ghcr.io/0xerr0r/blocky',
            ports: [{
                name: 'dns-udp',
                number: 53,
                protocol: Protocol.UDP,
            }, {
                name: "http",
                number: 4000,
                protocol: Protocol.TCP,
            }],
            envVariables: {
                TZ: { value: 'America/Los_Angeles' },
            },
            volumeMounts: [{
                volume: configVolume,
                path: '/app/config.yml',
                subPath: 'config.yaml',
            }],
        }
    }

}
