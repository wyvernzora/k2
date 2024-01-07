import {GlauthConfig} from "./config";
import {GlauthCertificate} from "./certificate";
import {Deployment, Protocol, Volume} from "cdk8s-plus-27";
import {Construct} from "constructs";
import {GlauthUsers} from "./users";


export interface GlauthDeploymentProps {
    readonly config: GlauthConfig;
    readonly users: GlauthUsers;
    readonly certificate: GlauthCertificate;
}

export class GlauthDeployment extends Deployment {

    constructor(scope: Construct, id: string, props: GlauthDeploymentProps) {
        super(scope, id, { replicas: 1 });
        const configVolume = Volume.fromConfigMap(this, 'config', props.config);
        const certVolume = Volume.fromSecret(this, 'cert', props.certificate.secret);
        const usersVolume = Volume.fromSecret(this, 'users', props.users.secret);
        this.addGlauthContainer(configVolume, certVolume, usersVolume);
    }

    private addGlauthContainer(config: Volume, cert: Volume, users: Volume): void {
        this.addContainer({
            name: 'glauth',
            image: 'glauth/glauth:v2.3.0',
            command: [
                '/app/glauth',
                '-c', '/app/conf.d/',
            ],
            securityContext: {
                ensureNonRoot: false,
                readOnlyRootFilesystem: false,
            },
            ports: [{
                name: 'ldap',
                number: 389,
                protocol: Protocol.TCP,
            }, {
                name: 'ldaps',
                number: 636,
                protocol: Protocol.TCP,
            }],
            volumeMounts: [{
                volume: config,
                path: '/app/conf.d/config.cfg',
                subPath: 'config.cfg',
            }, {
                volume: users,
                path: '/app/conf.d/users.cfg',
                subPath: 'users.conf',
            }, {
                volume: cert,
                path: '/app/tls/glauth.crt',
                subPath: 'tls.crt',
            }, {
                volume: cert,
                path: '/app/tls/glauth.key',
                subPath: 'tls.key',
            }],
        })
    }

}
