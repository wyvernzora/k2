set -eu
echo 'k2-tools: installing worker files'
sudo mkdir -p /etc/rancher/k3s/config.yaml.d /oem /home/kairos/.ssh
echo 'k2-tools: installing worker join config'
sudo install -m 0600 "/tmp/k2-remote"/30-k2-worker.yaml /etc/rancher/k3s/config.yaml.d/30-k2-worker.yaml
echo 'k2-tools: installing Kairos k3s activation cloud-config'
sudo install -m 0644 "/tmp/k2-remote"/99-k2-k3s-worker.yaml /oem/99-k2-k3s-worker.yaml
sudo install -m 0644 "/tmp/k2-remote"/98-k2-operator-keys.yaml /oem/98-k2-operator-keys.yaml
echo 'k2-tools: installing operator SSH keys'
sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh
sudo install -o kairos -g kairos -m 0600 "/tmp/k2-remote"/operator_authorized_keys /home/kairos/.ssh/authorized_keys
echo 'k2-tools: enabling k3s-agent service'
sudo systemctl enable k3s-agent
