import { App, HelmChart } from "@k2/cdk-lib";
import * as authelia from "@k2/authelia";

const AppHealthCustomization = `
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
  return hs`;

const app = new App();
new HelmChart(app, "argocd", {
  namespace: "k2-core",
  chart: "helm:https://argoproj.github.io/argo-helm/argo-cd@7.1.2",
  values: {
    secret: {
      createSecret: false,
    },
    server: {
      ingress: {
        enabled: true,
        annotations: {
          ...authelia.MiddlewareAnnotation,
        },
        hostname: "deploy.wyvernzora.io",
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
        "resource.customizations.health.argoproj.io_Application":
          AppHealthCustomization,
      },
    },
  },
});

app.synth();
