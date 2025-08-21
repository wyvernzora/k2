import { HelmChart } from "@k2/cdk-lib";
import { Construct } from "constructs";
import * as authelia from "@k2/authelia";
import dedent from "dedent-js";
import { ApexDomainContext } from "cdk-lib/context/apex-domain";

const AppHealthCustomization = dedent`
  hs = {}
  hs.status = "Progressing"
  hs.message = ""
  if obj.status ~= nil then
  if obj.status.health ~= nil then
      hs.status = obj.status.health.status
          if obj.status.health.message ~= nil then
              hs.message = obj.status.health.message
          end
  end
  end
  return hs
`;

export interface ArgoCdProps {
  readonly subdomain: string;
  readonly namespace: string;
}

export class ArgoCd extends HelmChart {
  constructor(scope: Construct, name: string, props: ArgoCdProps) {
    const apexDomain = ApexDomainContext.of(scope).domain;
    super(scope, name, {
      namespace: props.namespace,
      chart: "helm:https://argoproj.github.io/argo-helm/argo-cd@8.3.0",
      values: {
        secret: {
          createSecret: false,
        },
        server: {
          ingress: {
            enabled: true,
            annotations: {
              ...authelia.MiddlewareAnnotation,
              "traefik.ingress.kubernetes.io/router.tls": "true",
            },
            hostname: `${props.subdomain}.${apexDomain}`,
          },
        },
        configs: {
          params: {
            // Let ingress controller handle TLS termination
            "server.insecure": true,
            // Disable builtin auth and let Authelia handle it
            "server.disable.auth": true,
          },
          cm: {
            "statusbadge.enabled": true,
            "reposerver.enable.git.submodule": false,
            "resource.customizations.health.argoproj.io_Application": AppHealthCustomization,
          },
        },
      },
    });
  }
}
