package toolcli

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/kairos/tools/remote"
)

type storagePoolScriptInput struct {
	Pool          string
	ClusterName   string
	Compatibility string
	VDevs         []storageVDev
	ForceWipe     bool
	CreateAllowed bool
}

var remoteStorageMarkers = []string{
	"__K2_POOL_LIST__",
	"__K2_POOL_IMPORT__",
	"__K2_LSBLK__",
	"__K2_BYID__",
}

func inspectRemoteStorage(client *remote.Client, pool string) (storageInspection, error) {
	script := strings.Join([]string{
		"set -eu",
		"echo " + remoteStorageMarkers[0],
		fmt.Sprintf("sudo zpool list -H -o name,health %s 2>/dev/null || true", shellQuote(pool)),
		"echo " + remoteStorageMarkers[1],
		// `zpool import` has no -H/-o listing mode; parse the human output.
		"sudo zpool import 2>/dev/null | awk '/^ *pool: /{print $2}' || true",
		"echo " + remoteStorageMarkers[2],
		"lsblk -J -b -o NAME,TYPE,SIZE,MODEL,FSTYPE,LABEL,MOUNTPOINT",
		"echo " + remoteStorageMarkers[3],
		"ls -l /dev/disk/by-id 2>/dev/null || true",
	}, "\n")
	out, err := client.Capture(script)
	if err != nil {
		return storageInspection{}, fmt.Errorf("inspect remote storage: %w", err)
	}
	return parseRemoteStorageInspection(out, pool)
}

func parseRemoteStorageInspection(out []byte, pool string) (storageInspection, error) {
	sections := splitMarkedSections(string(out))
	list := strings.TrimSpace(sections[remoteStorageMarkers[0]])
	state := storagePoolMissing
	health := ""
	if list != "" {
		state = storagePoolImported
		fields := strings.Fields(list)
		if len(fields) >= 2 {
			health = fields[1]
		}
	} else if slices.Contains(strings.Fields(sections[remoteStorageMarkers[1]]), pool) {
		state = storagePoolImportable
	}
	disks, err := parseStorageDisks([]byte(sections[remoteStorageMarkers[2]]), []byte(sections[remoteStorageMarkers[3]]))
	if err != nil {
		return storageInspection{}, err
	}
	return storageInspection{
		PoolState:  state,
		PoolHealth: health,
		Disks:      disks,
	}, nil
}

func splitMarkedSections(out string) map[string]string {
	markers := map[string]bool{}
	for _, marker := range remoteStorageMarkers {
		markers[marker] = true
	}
	sections := map[string]string{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if markers[line] {
			current = line
			continue
		}
		if current != "" {
			sections[current] += line + "\n"
		}
	}
	return sections
}

func storageInstallScript(nodeName string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "remote_dir=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\"\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: setting live hostname'\n")
	fmt.Fprintf(&buf, "sudo hostnamectl set-hostname %s\n", shellQuote(nodeName))
	fmt.Fprintf(&buf, "grep -qxF %s /etc/hosts || echo %s | sudo tee -a /etc/hosts >/dev/null\n", shellQuote("127.0.1.1 "+nodeName), shellQuote("127.0.1.1 "+nodeName))
	fmt.Fprintf(&buf, "echo 'k2-tools: installing storage hostname activation cloud-config'\n")
	fmt.Fprintf(&buf, "sudo mkdir -p /oem /home/kairos/.ssh\n")
	fmt.Fprintf(&buf, "sudo install -m 0644 \"$remote_dir\"/99-k2-storage-hostname.yaml /oem/99-k2-storage-hostname.yaml\n")
	fmt.Fprintf(&buf, "if [ -s \"$remote_dir\"/operator_authorized_keys ]; then\n")
	fmt.Fprintf(&buf, "  echo 'k2-tools: installing operator SSH keys'\n")
	fmt.Fprintf(&buf, "  sudo install -d -o kairos -g kairos -m 0700 /home/kairos/.ssh\n")
	fmt.Fprintf(&buf, "  sudo install -o kairos -g kairos -m 0600 \"$remote_dir\"/operator_authorized_keys /home/kairos/.ssh/authorized_keys\n")
	fmt.Fprintf(&buf, "  echo 'k2-tools: installing reset-surviving operator key stage'\n")
	fmt.Fprintf(&buf, "  sudo install -m 0644 \"$remote_dir\"/98-k2-storage-operator-keys.yaml /oem/98-k2-storage-operator-keys.yaml\n")
	fmt.Fprintf(&buf, "else\n")
	fmt.Fprintf(&buf, "  echo 'k2-tools: no operator SSH keys supplied'\n")
	fmt.Fprintf(&buf, "fi\n")
	fmt.Fprintf(&buf, "echo 'k2-tools: ensuring csi user'\n")
	fmt.Fprintf(&buf, "if ! id csi >/dev/null 2>&1; then sudo useradd --create-home --shell /bin/sh csi; fi\n")
	fmt.Fprintf(&buf, "sudo install -d -o csi -g csi -m 0700 /home/csi/.ssh\n")
	fmt.Fprintf(&buf, "sudo install -o csi -g csi -m 0600 \"$remote_dir\"/csi_authorized_keys /home/csi/.ssh/authorized_keys\n")
	fmt.Fprintf(&buf, "sudo install -m 0440 \"$remote_dir\"/99-csi /etc/sudoers.d/99-csi\n")
	fmt.Fprintf(&buf, "sudo touch /home/csi/.hushlogin\n")
	fmt.Fprintf(&buf, "sudo chown csi:csi /home/csi/.hushlogin\n")
	return buf.String()
}

func storagePoolScript(in storagePoolScriptInput) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "set -eu\n")
	fmt.Fprintf(&buf, "pool=%s\n", shellQuote(in.Pool))
	fmt.Fprintf(&buf, "compat=%s\n", shellQuote(in.Compatibility))
	fmt.Fprintf(&buf, "cluster=%s\n", shellQuote(in.ClusterName))
	fmt.Fprintf(&buf, "echo 'k2-tools: provisioning ZFS pool and datasets'\n")
	fmt.Fprintf(&buf, "if sudo zpool list -H -o name \"$pool\" >/dev/null 2>&1; then\n")
	fmt.Fprintf(&buf, "  health=\"$(sudo zpool list -H -o health \"$pool\")\"\n")
	fmt.Fprintf(&buf, "  test \"$health\" = ONLINE || { echo \"pool $pool health is $health\" >&2; exit 1; }\n")
	fmt.Fprintf(&buf, "  echo \"k2-tools: pool $pool already imported ($health)\"\n")
	fmt.Fprintf(&buf, "elif sudo zpool import \"$pool\" >/dev/null 2>&1; then\n")
	fmt.Fprintf(&buf, "  echo \"k2-tools: imported existing pool $pool\"\n")
	fmt.Fprintf(&buf, "else\n")
	if !in.CreateAllowed {
		fmt.Fprintf(&buf, "  echo \"pool $pool missing at execution time\" >&2\n")
		fmt.Fprintf(&buf, "  exit 1\n")
	} else {
		fmt.Fprintf(&buf, "  sudo test -f \"/usr/share/zfs/compatibility.d/$compat\"\n")
		for _, dev := range storageVDevDevices(in.VDevs) {
			fmt.Fprintf(&buf, "  sudo test -b %s\n", shellQuote(dev))
			if in.ForceWipe {
				fmt.Fprintf(&buf, "  sudo wipefs -a %s\n", shellQuote(dev))
			} else {
				fmt.Fprintf(&buf, "  if sudo wipefs -n %s | grep -q . || sudo blkid %s >/dev/null 2>&1; then echo 'device %s is not blank; pass --force-wipe' >&2; exit 1; fi\n", shellQuote(dev), shellQuote(dev), dev)
			}
		}
		fmt.Fprintf(&buf, "  sudo zpool create -m none -o ashift=12 -o cachefile=none -o autotrim=on -o compatibility=\"$compat\" -O compression=lz4 -O atime=off -O canmount=off \"$pool\"")
		for _, arg := range storageZpoolVDevArgs(in.VDevs) {
			fmt.Fprintf(&buf, " %s", shellQuote(arg))
		}
		fmt.Fprintf(&buf, "\n")
	}
	fmt.Fprintf(&buf, "fi\n")
	fmt.Fprintf(&buf, "for ds in \"$pool/csi\" \"$pool/csi/$cluster\" \"$pool/csi/$cluster-snapshots\"; do\n")
	fmt.Fprintf(&buf, "  sudo zfs list -H -o name \"$ds\" >/dev/null 2>&1 || sudo zfs create -o canmount=off -o mountpoint=none \"$ds\"\n")
	fmt.Fprintf(&buf, "done\n")
	return buf.String()
}

func storageVDevDevices(vdevs []storageVDev) []string {
	var out []string
	for _, vdev := range vdevs {
		out = append(out, vdev.Devices...)
	}
	return out
}

func storageZpoolVDevArgs(vdevs []storageVDev) []string {
	var out []string
	for _, vdev := range vdevs {
		if vdev.Topology != "" {
			out = append(out, vdev.Topology)
		}
		out = append(out, vdev.Devices...)
	}
	return out
}
