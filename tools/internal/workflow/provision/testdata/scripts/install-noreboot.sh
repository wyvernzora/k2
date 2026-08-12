set -eu
echo 'k2-tools: installing bootstrap files'
sudo mkdir -p /etc/rancher/k3s/config.yaml.d /var/lib/rancher/k3s/server/manifests /oem /home/kairos/.ssh
echo 'k2-tools: activating k3s server invariants'
sudo cp /usr/share/k2/node-provision/k3s/10-k2-invariant.yaml /etc/rancher/k3s/config.yaml.d/10-k2-invariant.yaml
echo 'k2-tools: disabling unwanted k3s packaged manifests'
sudo touch /var/lib/rancher/k3s/server/manifests/traefik.yaml.skip
sudo touch /var/lib/rancher/k3s/server/manifests/local-storage.yaml.skip
sudo touch /var/lib/rancher/k3s/server/manifests/metrics-server.yaml.skip
echo 'k2-tools: installing cluster and bootstrap config'
sudo install -m 0644 "/tmp/k2-remote"/20-k2-cluster.yaml /etc/rancher/k3s/config.yaml.d/20-k2-cluster.yaml
sudo install -m 0644 "/tmp/k2-remote"/30-k2-bootstrap.yaml /etc/rancher/k3s/config.yaml.d/30-k2-bootstrap.yaml
echo 'k2-tools: installing Kairos k3s activation cloud-config'
sudo install -m 0644 "/tmp/k2-remote"/99-k2-k3s-bootstrap.yaml /oem/99-k2-k3s-bootstrap.yaml
sudo install -m 0644 "/tmp/k2-remote"/98-k2-operator-keys.yaml /oem/98-k2-operator-keys.yaml
echo 'k2-tools: installing bootstrap manifest bundle'
sudo install -m 0644 "/tmp/k2-remote"/k2-bootstrap.k8s.yaml /var/lib/rancher/k3s/server/manifests/k2-bootstrap.yaml
echo 'k2-tools: staging root Argo CD app manifest'
sudo install -m 0644 "/tmp/k2-remote"/k2-root-argocd-app.k8s.yaml /var/lib/rancher/k3s/server/k2-root-argocd-app.k8s.yaml
echo 'k2-tools: installing operator SSH keys'
sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh
sudo install -o kairos -g kairos -m 0600 "/tmp/k2-remote"/operator_authorized_keys /home/kairos/.ssh/authorized_keys
echo 'k2-tools: enabling k3s service'
sudo systemctl enable k3s
