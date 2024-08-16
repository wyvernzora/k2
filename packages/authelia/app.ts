import { App, HelmChart } from "@k2/cdk-lib";
import { OnePasswordItem } from "@k2/1password/crds";
import { Middleware } from "@k2/traefik/crds";

const app = new App();
const chart = new HelmChart(app, "authelia", {
  namespace: "k2-auth",
  chart: "helm:https://charts.authelia.com/authelia@0.9.5",
  values: {
    domain: "wyvernzora.io",
    ingress: {
      enabled: true,
      tls: {
        enabled: true,
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
      default_redirection_url: "https://h.wyvernzora.io",
      authentication_backend: {
        ldap: {
          enabled: true,
          implementation: "custom",
          url: "ldap://glauth.k2-auth.svc.cluster.local",
          tls: {
            server_name: "ldap.wyvernzora.io",
          },
          base_dn: "dc=wyvernzora,dc=io",
          user: "cn=authelia,dc=wyvernzora,dc=io",
          additional_users_dn: "ou=users",
          users_filter:
            "(&(|({username_attribute}={input})({mail_attribute}={input}))(objectClass=posixAccount))",
          additional_groups_dn: "ou=groups",
          groups_filter: "(&(memberUid={username})(objectClass=posixGroup))",
        },
      },
      access_control: {
        default_policy: "one_factor",
        networks: [
          {
            name: "private",
            networks: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"],
          },
        ],
      },
      session: {
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

/**
 * Inject the secret
 */
new OnePasswordItem(chart, "secret", {
  metadata: {
    name: "authelia",
  },
  spec: {
    itemPath:
      "vaults/zfsyjjcwge4w4gw6dh4zaqndhq/items/ejfcz3g4s6wsr2jtct6hs3alxi",
  },
});

/**
 * Traefik middleware
 */
new Middleware(chart, "middleware", {
  metadata: {
    name: "authelia",
  },
  spec: {
    forwardAuth: {
      address:
        "http://authelia.k2-auth.svc.cluster.local/api/verify?rd=https%3A%2F%2Fauth.wyvernzora.io%2F",
      authResponseHeaders: [
        "Remote-User",
        "Remote-Groups",
        "Remote-Email",
        "Remote-Name",
      ],
    },
  },
});

app.synth();
