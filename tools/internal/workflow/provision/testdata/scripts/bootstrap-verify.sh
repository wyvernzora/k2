set -eu
verify() { label="$1"; shift; echo "k2-tools: verify: ${label}"; "$@"; }
verify 'hostname set' test "$(hostname)" = 'k2-node'
verify 'operator authorized keys installed' sudo test -s /home/kairos/.ssh/authorized_keys
verify 'server invariant config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml
verify 'cluster config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml
verify 'bootstrap config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/30-k2-bootstrap.yaml
verify 'bootstrap activation installed' sudo test -s /oem/99-k2-k3s-bootstrap.yaml
verify 'operator activation installed' sudo test -s /oem/98-k2-operator-keys.yaml
verify 'k3s service enabled' systemctl is-enabled --quiet k3s
verify 'k3s service active' systemctl is-active --quiet k3s
verify 'k3s kubeconfig exists' sudo test -s /etc/rancher/k3s/k3s.yaml
verify 'server token exists' sudo test -s /var/lib/rancher/k3s/server/token
verify 'node token exists' sudo test -s /var/lib/rancher/k3s/server/node-token
verify 'agent token exists' sudo test -s /var/lib/rancher/k3s/server/agent-token
verify 'root Argo CD app manifest staged' sudo test -s /var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml
verify 'root Argo CD Application applied' sudo kubectl -n argocd get application k2 >/dev/null
verify 'traefik packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip
verify 'local-storage packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip
verify 'metrics-server packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip
