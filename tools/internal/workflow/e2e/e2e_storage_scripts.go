package e2e

import (
	"embed"

	"github.com/wyvernzora/k2/tools/internal/shellscript"
)

//go:embed scripts/*.sh.tmpl
var scriptTemplates embed.FS

var scripts = shellscript.New(scriptTemplates, "scripts/*.sh.tmpl")

func e2eNodeISCSIPrepScript() string {
	return scripts.Render("node-iscsi-prep.sh.tmpl", nil)
}

func e2eStorageConsistencyScript(creds storageCredentials, pvName string, expectedBytes int64) string {
	return scripts.Render("storage-consistency.sh.tmpl", map[string]any{
		"DatasetParentName": creds.DatasetParentName,
		"Pool":              creds.Pool,
		"IQNBase":           creds.IQNBase,
		"PVName":            pvName,
		"ExpectedBytes":     expectedBytes,
	})
}

func e2eStorageCleanupPollScript(creds storageCredentials, pvName string) string {
	return scripts.Render("storage-cleanup-poll.sh.tmpl", map[string]any{
		"DatasetParentName":                  creds.DatasetParentName,
		"DetachedSnapshotsDatasetParentName": creds.DetachedSnapshotsDatasetParentName,
		"PVName":                             pvName,
	})
}

// e2eNodeNoISCSISessionScript asserts no session against the appliance
// remains. iscsiadm needs root, and "no sessions" is exit 21 — every other
// failure must fail the check rather than read as "no sessions found".
func e2eNodeNoISCSISessionScript(iqnBase string) string {
	return scripts.Render("node-no-iscsi-session.sh.tmpl", map[string]any{
		"IQNBase": iqnBase,
	})
}
