import dedent from "dedent-js";

import { ApexDomain, AppResourceFunc, ArgoCDResourceFunc, defineDeployment, HelmCharts } from "@k2/cdk-lib";
import * as Auth from "@k2/auth";

import { ContinuousDeployment } from "./lib/cd.js";

/* Export raw CRDs */
export * as crd from "./crds/argoproj.io.js";

/* Export higher level constructs */
export * from "./lib/cd.js";
export * from "./lib/context.js";

export const deployment = defineDeployment({
  targets: {
    legacy: true,
  },
});

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
      dex: {
        enabled: false,
      },
      notifications: {
        enabled: false,
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
          "resource.customizations.health.cnpg.wyvernzora.io_DatabaseClaim": dedent`
            hs = { status = "Progressing", message = "Waiting for DatabaseClaim status" }

            if obj.status ~= nil and obj.status.conditions ~= nil then
              for _, condition in ipairs(obj.status.conditions) do
                if condition.type == "Ready" then
                  hs.message = condition.message or condition.reason or ""

                  if condition.status == "True" then
                    hs.status = "Healthy"
                    return hs
                  end

                  if condition.status == "False" then
                    if condition.reason == "ClusterNotReady" or condition.reason == "Reconciling" then
                      hs.status = "Progressing"
                    else
                      hs.status = "Degraded"
                    end
                    return hs
                  end
                end
              end
            end

            return hs
          `,
          "resource.customizations.health.cnpg.wyvernzora.io_RoleClaim": dedent`
            hs = { status = "Progressing", message = "Waiting for RoleClaim status" }

            if obj.status ~= nil and obj.status.conditions ~= nil then
              for _, condition in ipairs(obj.status.conditions) do
                if condition.type == "Ready" then
                  hs.message = condition.message or condition.reason or ""

                  if condition.status == "True" then
                    hs.status = "Healthy"
                    return hs
                  end

                  if condition.status == "False" then
                    if condition.reason == "DatabaseNotReady" or condition.reason == "ClusterNotReady" or condition.reason == "Reconciling" then
                      hs.status = "Progressing"
                    else
                      hs.status = "Degraded"
                    end
                    return hs
                  end
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
