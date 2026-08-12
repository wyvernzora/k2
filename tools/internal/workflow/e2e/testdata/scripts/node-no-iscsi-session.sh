set -eu
out="$(sudo iscsiadm -m session 2>&1)" && rc=0 || rc=$?
if [ "$rc" -ne 0 ] && [ "$rc" -ne 21 ]; then echo "iscsiadm failed ($rc): $out" >&2; exit 1; fi
if printf '%s\n' "$out" | grep 'iqn.2026-07.io.wyvernzora.k2:storage'; then exit 1; fi
