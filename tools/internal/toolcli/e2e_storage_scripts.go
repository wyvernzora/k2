package toolcli

import (
	"bytes"
	"fmt"
)

func e2eNodeISCSIPrepScript() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: preparing iSCSI initiator'\n")
	// ponytail: phase-6 overlay work should replace this imperative VM-only workaround.
	fmt.Fprintf(&buf, "new_iqn=\"$(iscsi-iname)\"\n")
	fmt.Fprintf(&buf, "printf 'InitiatorName=%%s\\n' \"$new_iqn\" | sudo tee /etc/iscsi/initiatorname.iscsi >/dev/null\n")
	fmt.Fprintf(&buf, "sudo systemctl enable --now iscsid\n")
	fmt.Fprintf(&buf, "sudo systemctl restart iscsid\n")
	return buf.String()
}

func e2eStorageConsistencyScript(creds storageCredentials, pvName string, expectedBytes int64) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "zvol=\"$(sudo zfs list -H -o name -r %s | grep %s | head -n1)\"\n", shellQuote(creds.DatasetParentName), shellQuote(pvName))
	fmt.Fprintf(&buf, "test -n \"$zvol\"\n")
	fmt.Fprintf(&buf, "volsize=\"$(sudo zfs get -Hp -o value volsize \"$zvol\")\"\n")
	fmt.Fprintf(&buf, "test \"$volsize\" -ge %d\n", expectedBytes)
	// keystatus must be exactly 'available': '-' means unencrypted, and
	// 'unavailable' means encrypted but NOT unlocked — both are failures.
	fmt.Fprintf(&buf, "test \"$(sudo zfs get -H -o value keystatus %s)\" = 'available'\n", shellQuote(creds.Pool))
	fmt.Fprintf(&buf, "test \"$(sudo zfs get -H -o value encryption \"$zvol\")\" != 'off'\n")
	fmt.Fprintf(&buf, "sudo targetcli ls /iscsi | grep %s >/dev/null\n", shellQuote(creds.IQNBase))
	fmt.Fprintf(&buf, "sudo targetcli ls /iscsi | grep %s >/dev/null\n", shellQuote(pvName))
	return buf.String()
}

func e2eStorageCleanupPollScript(creds storageCredentials, pvName string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "for i in $(seq 1 60); do\n")
	fmt.Fprintf(&buf, "  zvols=\"$(sudo zfs list -H -o name -r %s 2>/dev/null | grep %s || true)\"\n", shellQuote(creds.DatasetParentName), shellQuote(pvName))
	fmt.Fprintf(&buf, "  targets=\"$(sudo targetcli ls /iscsi 2>/dev/null | grep %s || true)\"\n", shellQuote(pvName))
	fmt.Fprintf(&buf, "  children=\"$(sudo zfs list -H -o name -r %s 2>/dev/null | tail -n +2 || true)\"\n", shellQuote(creds.DatasetParentName))
	fmt.Fprintf(&buf, "  snapchildren=\"$(sudo zfs list -H -o name -r %s 2>/dev/null | tail -n +2 || true)\"\n", shellQuote(creds.DetachedSnapshotsDatasetParentName))
	fmt.Fprintf(&buf, "  if [ -z \"$zvols\" ] && [ -z \"$targets\" ] && [ -z \"$children\" ] && [ -z \"$snapchildren\" ]; then exit 0; fi\n")
	fmt.Fprintf(&buf, "  echo \"k2-tools: waiting for democratic-csi cleanup attempt $i\"\n")
	fmt.Fprintf(&buf, "  sleep 5\n")
	fmt.Fprintf(&buf, "done\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: remaining zvols:' >&2\n")
	fmt.Fprintf(&buf, "sudo zfs list -H -o name -r %s >&2 || true\n", shellQuote(creds.DatasetParentName))
	fmt.Fprintf(&buf, "echo 'k2-tools: remaining targets:' >&2\n")
	fmt.Fprintf(&buf, "sudo targetcli ls /iscsi >&2 || true\n")
	fmt.Fprintf(&buf, "exit 1\n")
	return buf.String()
}

// e2eNodeNoISCSISessionScript asserts no session against the appliance
// remains. iscsiadm needs root, and "no sessions" is exit 21 — every other
// failure must fail the check rather than read as "no sessions found".
func e2eNodeNoISCSISessionScript(iqnBase string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "out=\"$(sudo iscsiadm -m session 2>&1)\" && rc=0 || rc=$?\n")
	fmt.Fprintf(&buf, "if [ \"$rc\" -ne 0 ] && [ \"$rc\" -ne 21 ]; then echo \"iscsiadm failed ($rc): $out\" >&2; exit 1; fi\n")
	fmt.Fprintf(&buf, "if printf '%%s\\n' \"$out\" | grep %s; then exit 1; fi\n", shellQuote(iqnBase))
	return buf.String()
}
