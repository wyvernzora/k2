package provision

import (
	"fmt"
	"slices"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/client/remote"
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

func storageInstallScript(nodeName string, hasNetwork bool, hasBackup bool) string {
	return renderScript("storage-install.sh.tmpl", map[string]any{
		"NodeName":   nodeName,
		"HostsEntry": "127.0.1.1 " + nodeName,
		"HasNetwork": hasNetwork,
		"HasBackup":  hasBackup,
	})
}

func storagePoolScript(in storagePoolScriptInput) string {
	return renderScript("storage-pool.sh.tmpl", map[string]any{
		"Pool":          in.Pool,
		"Compatibility": in.Compatibility,
		"ClusterName":   in.ClusterName,
		"CreateAllowed": in.CreateAllowed,
		"ForceWipe":     in.ForceWipe,
		"Devices":       storageVDevDevices(in.VDevs),
		"VDevArgs":      storageZpoolVDevArgs(in.VDevs),
	})
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
