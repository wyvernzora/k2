set -eu
for i in $(seq 1 60); do
  zvols="$(sudo zfs list -H -o name -r 'tank/csi/k2' 2>/dev/null | grep 'pvc-abc123' || true)"
  targets="$(sudo targetcli ls /iscsi 2>/dev/null | grep 'pvc-abc123' || true)"
  children="$(sudo zfs list -H -o name -r 'tank/csi/k2' 2>/dev/null | tail -n +2 || true)"
  snapchildren="$(sudo zfs list -H -o name -r 'tank/csi/k2-snapshots' 2>/dev/null | tail -n +2 || true)"
  if [ -z "$zvols" ] && [ -z "$targets" ] && [ -z "$children" ] && [ -z "$snapchildren" ]; then exit 0; fi
  echo "k2-tools: waiting for democratic-csi cleanup attempt $i"
  sleep 5
done
echo 'k2-tools: remaining zvols:' >&2
sudo zfs list -H -o name -r 'tank/csi/k2' >&2 || true
echo 'k2-tools: remaining targets:' >&2
sudo targetcli ls /iscsi >&2 || true
exit 1
