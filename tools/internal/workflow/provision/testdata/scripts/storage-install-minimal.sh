set -eu
remote_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
echo 'k2-tools: setting live hostname'
sudo hostnamectl set-hostname 'k2-storage'
grep -qxF '127.0.1.1 k2-storage' /etc/hosts || echo '127.0.1.1 k2-storage' | sudo tee -a /etc/hosts >/dev/null
echo 'k2-tools: installing storage hostname activation cloud-config'
sudo mkdir -p /oem /home/kairos/.ssh
sudo install -m 0644 "$remote_dir"/99-k2-storage-hostname.yaml /oem/99-k2-storage-hostname.yaml
if [ -s "$remote_dir"/operator_authorized_keys ]; then
  echo 'k2-tools: installing operator SSH keys'
  sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh
  sudo install -o kairos -g kairos -m 0600 "$remote_dir"/operator_authorized_keys /home/kairos/.ssh/authorized_keys
  echo 'k2-tools: installing reset-surviving operator key stage'
  sudo install -m 0644 "$remote_dir"/98-k2-storage-operator-keys.yaml /oem/98-k2-storage-operator-keys.yaml
else
  echo 'k2-tools: no operator SSH keys supplied'
fi
echo 'k2-tools: installing k2-csi key'
if ! id k2-csi >/dev/null 2>&1; then
  echo 'account k2-csi is missing; this image predates the storage-users action - upgrade the appliance image first' >&2
  exit 1
fi
uid="$(id -u k2-csi)"
clash="$(getent passwd | awk -F: -v u="$uid" -v a=k2-csi '$3 == u && $1 != a {print $1}')"
if [ -n "$clash" ]; then
  echo "account k2-csi shares uid $uid with: $clash - refusing to install a key onto a shared identity; rebuild the image with pinned uids" >&2
  exit 1
fi
sudo install -d -o k2-csi -g k2-csi -m 0755 /home/k2-csi
sudo chown -R k2-csi:k2-csi /home/k2-csi
sudo install -d -o k2-csi -g k2-csi -m 0700 /home/k2-csi/.ssh
sudo install -o k2-csi -g k2-csi -m 0600 "$remote_dir"/csi_authorized_keys /home/k2-csi/.ssh/authorized_keys
echo 'k2-tools: installing snapshot cadence config'
sudo install -m 0644 "$remote_dir"/k2-snapshot.env /oem/k2-snapshot.env
