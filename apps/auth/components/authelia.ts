import { ApexDomain, App, HelmCharts } from "@k2/cdk-lib";
import { K2Secret } from "@k2/1password";
import * as Traefik from "@k2/traefik";

export default {
  create(app: App) {
    const helm = HelmCharts.of(app);
    const Authelia = helm.asChart("authelia");

    const domainContext = ApexDomain.of(app);

    const chart = new Authelia(app, "authelia", {
      namespace: "auth",
      values: {
        ingress: {
          enabled: true,
          tls: {
            enabled: true,
            secret: "default-certificate",
          },
        },
        pod: {
          kind: "Deployment",
          extraVolumes: [
            {
              name: "ephemeral",
              emptyDir: {},
            },
          ],
          extraVolumeMounts: [
            {
              name: "ephemeral",
              mountPath: "/var/authelia",
            },
          ],
        },
        configMap: {
          definitions: {
            network: {
              private: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"],
            },
          },
          authentication_backend: {
            ldap: {
              enabled: true,
              implementation: "glauth",
              address: "ldap://glauth.auth.svc.cluster.local",
              tls: {
                server_name: domainContext.subdomain("ldap"),
              },
              base_dn: "dc=wyvernzora,dc=io",
              additional_users_dn: "ou=users",
              user: "cn=authelia,dc=wyvernzora,dc=io",
            },
          },
          access_control: {
            default_policy: "one_factor",
          },
          session: {
            cookies: [
              {
                domain: domainContext.apexDomain,
                subdomain: "auth",
                default_redirection_url: `https://${domainContext.subdomain("h")}`,
              },
            ],
            redis: {
              enabled: false,
            },
          },
          storage: {
            local: {
              enabled: true,
              path: "/var/authelia/db.sqlite3",
            },
            postgres: {
              enabled: false,
            },
          },
          notifier: {
            filesystem: {
              enabled: true,
              filename: "/var/authelia/notifications.log",
            },
            smtp: {
              enabled: false,
            },
          },
        },
        secret: {
          existingSecret: "authelia",
        },
      },
    });

    new K2Secret(chart, "secret", {
      metadata: {
        name: "authelia",
      },
      itemId: "ejfcz3g4s6wsr2jtct6hs3alxi",
    });

    new Traefik.crd.Middleware(chart, "middleware", {
      metadata: {
        name: "authelia",
      },
      spec: {
        forwardAuth: {
          address:
            "http://authelia.auth.svc.cluster.local/api/authz/forward-auth?rd=https%3A%2F%2Fauth.wyvernzora.io%2F",
          authResponseHeaders: ["Remote-User", "Remote-Groups", "Remote-Email", "Remote-Name"],
        },
      },
    });
  },
};
