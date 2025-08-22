import { HelmChart } from "@k2/cdk-lib";
import * as OnePassword from "@k2/1password";
import * as Traefik from "@k2/traefik";
import { Construct } from "constructs";

export class Authelia extends Construct {
  readonly helm: HelmChart;

  constructor(scope: Construct, name: string) {
    super(scope, name);

    this.helm = new HelmChart(this, "authelia", {
      namespace: "auth",
      chart: "helm:https://charts.authelia.com/authelia@0.10.42",
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
                server_name: "ldap.wyvernzora.io",
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
                domain: "wyvernzora.io",
                subdomain: "auth",
                default_redirection_url: "https://h.wyvernzora.io",
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

    new OnePassword.K2Secret(this.helm, "secret", {
      metadata: {
        name: "authelia",
      },
      itemId: "ejfcz3g4s6wsr2jtct6hs3alxi",
    });

    new Traefik.crd.Middleware(this.helm, "middleware", {
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
  }
}
