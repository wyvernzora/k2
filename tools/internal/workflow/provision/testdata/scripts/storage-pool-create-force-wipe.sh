set -eu
remote_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
pool='tank'
compat='openzfs-2.2-linux'
cluster='k2'
key_dir=/usr/local/.state/zfs
key_file="$key_dir/$pool.key"
echo 'k2-tools: provisioning ZFS pool and datasets'
sudo install -d -o root -g root -m 0700 "$key_dir"
verify_key() { sudo zfs load-key -n -L "file://$remote_dir/zfs_pool.key" "$pool" >/dev/null 2>&1 || { echo "escrowed key does not match encrypted pool $pool; refusing to overwrite $key_file (recover the original storage-appliance.json or destroy the pool first)" >&2; exit 1; }; }
install_key() { sudo install -o root -g root -m 0400 "$remote_dir"/zfs_pool.key "$key_file"; }
if sudo zpool list -H -o name "$pool" >/dev/null 2>&1; then
  health="$(sudo zpool list -H -o health "$pool")"
  test "$health" = ONLINE || { echo "pool $pool health is $health" >&2; exit 1; }
  verify_key
  install_key
  echo "k2-tools: pool $pool already imported ($health)"
elif sudo zpool import "$pool" >/dev/null 2>&1; then
  echo "k2-tools: imported existing pool $pool"
  verify_key
  install_key
  if [ "$(sudo zfs get -H -o value keystatus "$pool")" != available ]; then sudo zfs load-key "$pool"; fi
else
  install_key
  sudo test -f "/usr/share/zfs/compatibility.d/$compat"
  sudo test -b '/dev/disk/by-id/ata-a'
  sudo wipefs -a '/dev/disk/by-id/ata-a'
  sudo test -b '/dev/disk/by-id/ata-b'
  sudo wipefs -a '/dev/disk/by-id/ata-b'
  sudo zpool create -m none -o ashift=12 -o cachefile=none -o autotrim=on -o compatibility="$compat" -O compression=lz4 -O atime=off -O canmount=off -O encryption=aes-256-gcm -O keyformat=raw -O keylocation="file://$key_file" "$pool" 'mirror' '/dev/disk/by-id/ata-a' '/dev/disk/by-id/ata-b'
fi
for ds in "$pool/csi" "$pool/csi/$cluster" "$pool/csi/$cluster-snapshots"; do
  sudo zfs list -H -o name "$ds" >/dev/null 2>&1 || sudo zfs create -o canmount=off -o mountpoint=none "$ds"
done
