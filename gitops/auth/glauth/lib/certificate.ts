import { Certificate } from "~crds/cert-manager.io";
import {Construct} from "constructs";
import {ISecret, Secret} from "cdk8s-plus-27";

export interface GlauthCertificateProps {
    readonly domain: string
}

export class GlauthCertificate extends Certificate {

    constructor(scope: Construct, id: string, props: GlauthCertificateProps) {
        super(scope, id, {
            spec: {
                commonName: `*.${props.domain}`,
                dnsNames: [`*.${props.domain}`],
                issuerRef: {
                    name: "letsencrypt-prod",
                    kind: "ClusterIssuer"
                },
                secretName: "glauth-tls"
            }
        });
    }
    public get secret(): ISecret {
        return Secret.fromSecretName(this, "glauth-tls", "glauth-tls");
    }
}
