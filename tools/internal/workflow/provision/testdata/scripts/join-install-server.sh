set -eu
echo 'k2-tools: installing server files'
sudo mkdir -p /etc/rancher/k3s/config.yaml.d /oem /home/kairos/.ssh
sudo mkdir -p /var/lib/rancher/k3s/server/manifests
echo 'k2-tools: activating k3s server invariants'
sudo cp /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml
echo 'k2-tools: disabling unwanted k3s packaged manifests'
sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip
sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip
sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip
echo 'k2-tools: installing cluster config'
sudo install -m 0644 "/tmp/k2-remote"/20-k2-cluster.yaml /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml
echo 'k2-tools: installing server join config'
sudo install -m 0600 "/tmp/k2-remote"/30-k2-server.yaml /etc/rancher/k3s/config.yaml.d/30-k2-server.yaml
echo 'k2-tools: installing Kairos k3s activation cloud-config'
sudo install -m 0644 "/tmp/k2-remote"/99-k2-k3s-server.yaml /oem/99-k2-k3s-server.yaml
sudo install -m 0644 "/tmp/k2-remote"/98-k2-operator-keys.yaml /oem/98-k2-operator-keys.yaml
echo 'k2-tools: installing static network cloud-config'
sudo install -m 0644 "/tmp/k2-remote"/97-k2-network.yaml /oem/97-k2-network.yaml
echo 'k2-tools: installing operator SSH keys'
sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh
sudo install -o kairos -g kairos -m 0600 "/tmp/k2-remote"/operator_authorized_keys /home/kairos/.ssh/authorized_keys
echo 'k2-tools: enabling k3s service'
sudo systemctl enable k3s
echo 'k2-tools: rebooting node'
sudo reboot
