set -eu
verify() { label="$1"; shift; echo "k2-tools: verify: ${label}"; "$@"; }
verify 'hostname set' test "$(hostname)" = 'k2-node'
verify 'operator authorized keys installed' sudo test -s /home/kairos/.ssh/authorized_keys
verify 'server join config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/30-k2-server.yaml
verify 'server activation installed' sudo test -s /oem/99-k2-k3s-server.yaml
verify 'operator activation installed' sudo test -s /oem/98-k2-operator-keys.yaml
verify 'network activation installed' sudo test -s /oem/97-k2-network.yaml
verify 'static network applied' ls /etc/systemd/network/10-k2-*.network
verify 'server invariant config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml
verify 'cluster config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml
verify 'k3s service enabled' systemctl is-enabled --quiet k3s
verify 'k3s service active' systemctl is-active --quiet k3s
verify 'k3s kubeconfig exists' sudo test -s /etc/rancher/k3s/k3s.yaml
verify 'traefik packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip
verify 'local-storage packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip
verify 'metrics-server packaged manifest disabled' sudo test -f /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip
