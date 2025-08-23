import dedent from "dedent-js";
import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, HelmCharts } from "@k2/cdk-lib";
import * as Auth from "@k2/auth";

/* Export raw CRDs */
import * as CRD from "./crds/argoproj.io";
import { ContinuousDeployment } from "./lib/cd";
export const crd = {
  ...CRD,
};

/* Export higher level constructs */
export * from "./lib/cd";
export * from "./lib/context";

/* Export deployment chart factory */
export const createAppResources: AppResourceFunc = app => {
  const helm = HelmCharts.of(app);
  const ArgoCD = helm.asChart("argo-cd");

  new ArgoCD(app, "argocd", {
    namespace: "k2-core",
    values: {
      secret: {
        createSecret: false,
      },
      server: {
        ingress: {
          enabled: true,
          annotations: {
            ...Auth.MiddlewareAnnotation,
            "traefik.ingress.kubernetes.io/router.tls": "true",
          },
          hostname: ApexDomain.of(app).subdomain("deploy"),
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
          "resource.customizations.health.argoproj.io_Application": dedent`
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
          `,
        },
      },
    },
  });
};

/* Export ArgoCD application factory */
export const createArgoCdResources: ArgoCDResourceFunc = chart => {
  new ContinuousDeployment(chart, "argocd", { namespace: "k2-core" });
};
