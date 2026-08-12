set -eu
zvol="$(sudo zfs list -H -o name -r 'tank/csi/k2' | grep 'pvc-abc123' | head -n1)"
test -n "$zvol"
volsize="$(sudo zfs get -Hp -o value volsize "$zvol")"
test "$volsize" -ge 1073741824
test "$(sudo zfs get -H -o value keystatus 'tank')" = 'available'
test "$(sudo zfs get -H -o value encryption "$zvol")" != 'off'
sudo targetcli ls /iscsi | grep 'iqn.2026-07.io.wyvernzora.k2:storage' >/dev/null
sudo targetcli ls /iscsi | grep 'pvc-abc123' >/dev/null
