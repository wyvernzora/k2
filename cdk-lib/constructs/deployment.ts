import dedent from "dedent-js";
import { Deployment, DeploymentProps, ISecret, Volume } from "cdk8s-plus-32";
import { Construct } from "constructs";

import { ConfigMap } from "./config-map.js";

const CADDY_TLS_PATH = "/etc/caddy/tls";

/**
 * Common K2 deployment base class.
 *
 * Extends the cdk8s-plus Deployment with homelab-specific sidecar helpers that
 * keep application deployments terse while preserving normal Deployment APIs.
 */
export class K2Deployment extends Deployment {
  constructor(scope: Construct, id: string, props?: DeploymentProps) {
    super(scope, id, props);
  }

  /**
   * Adds a Caddy sidecar that terminates TLS on pod port 443 and proxies to a
   * local plaintext application port.
   *
   * The provided secret must contain Kubernetes TLS keys named `tls.crt` and
   * `tls.key`. The sidecar watches those mounted files and force-reloads Caddy
   * when they change so cert-manager certificate rotations are picked up without
   * restarting the pod.
   */
  public addTLSTerminationProxy(targetPort: number, certSecret: ISecret): void {
    const config = new ConfigMap(this, "tls-termination-caddy-conf", {
      data: {
        Caddyfile: this.renderCaddyfile(targetPort),
        "watch-certs.sh": this.renderReloadScript(),
      },
    });
    config.addChecksumTo(this);

    this.addContainer({
      name: "caddy-tls-termination",
      image: "caddy:2-alpine",
      command: ["/bin/sh", "/etc/caddy/watch-certs.sh"],
      ports: [
        {
          name: "http",
          number: 80,
        },
        {
          name: "https",
          number: 443,
        },
      ],
      volumeMounts: [
        {
          volume: Volume.fromConfigMap(this, "tls-termination-caddy-conf-vol", config),
          path: "/etc/caddy",
          readOnly: true,
        },
        {
          volume: Volume.fromSecret(this, "tls-termination-cert-vol", certSecret),
          path: CADDY_TLS_PATH,
          readOnly: true,
        },
      ],
      securityContext: {
        ensureNonRoot: false,
        readOnlyRootFilesystem: false,
      },
    });
  }

  private renderCaddyfile(targetPort: number): string {
    return dedent`
      {
        auto_https off
      }

      :80 {
        redir https://{host}{uri} permanent
      }

      :443 {
        tls ${CADDY_TLS_PATH}/tls.crt ${CADDY_TLS_PATH}/tls.key

        reverse_proxy 127.0.0.1:${targetPort}
      }
    `;
  }

  private renderReloadScript(): string {
    return dedent`
      #!/bin/sh
      set -eu

      checksum() {
        sha256sum ${CADDY_TLS_PATH}/tls.crt ${CADDY_TLS_PATH}/tls.key 2>/dev/null || true
      }

      caddy run --config /etc/caddy/Caddyfile --adapter caddyfile &
      pid="$!"
      trap 'kill -TERM "$pid"; wait "$pid"' TERM INT

      last="$(checksum)"
      while kill -0 "$pid" 2>/dev/null; do
        sleep 60
        next="$(checksum)"
        if [ "$next" != "$last" ]; then
          caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile --force
          last="$next"
        fi
      done

      wait "$pid"
    `;
  }
}
