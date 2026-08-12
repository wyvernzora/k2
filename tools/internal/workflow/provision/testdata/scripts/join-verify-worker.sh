set -eu
verify() { label="$1"; shift; echo "k2-tools: verify: ${label}"; "$@"; }
verify 'hostname set' test "$(hostname)" = 'k2-node'
verify 'operator authorized keys installed' sudo test -s /home/kairos/.ssh/authorized_keys
verify 'worker join config installed' sudo test -s /etc/rancher/k3s/config.yaml.d/30-k2-worker.yaml
verify 'worker activation installed' sudo test -s /oem/99-k2-k3s-worker.yaml
verify 'operator activation installed' sudo test -s /oem/98-k2-operator-keys.yaml
verify 'server invariant config absent on worker' sudo test ! -e /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml
verify 'cluster config absent on worker' sudo test ! -e /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml
verify 'k3s-agent service enabled' systemctl is-enabled --quiet k3s-agent
verify 'k3s-agent service active' systemctl is-active --quiet k3s-agent
