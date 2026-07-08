package toolcli

import (
	"strings"
	"testing"
)

func TestParseRemoteStorageInspectionPoolStates(t *testing.T) {
	base := strings.Join([]string{
		"__K2_COMPAT__",
		"yes",
		"__K2_LSBLK__",
		`{"blockdevices":[]}`,
		"__K2_BYID__",
		"",
	}, "\n")
	tests := []struct {
		name string
		out  string
		want storagePoolState
	}{
		{
			name: "imported pool",
			out: strings.Join([]string{
				"__K2_POOL_LIST__",
				"tank ONLINE",
				"__K2_POOL_IMPORT__",
				"",
				base,
			}, "\n"),
			want: storagePoolImported,
		},
		{
			name: "requested importable pool among others",
			out: strings.Join([]string{
				"__K2_POOL_LIST__",
				"",
				"__K2_POOL_IMPORT__",
				"other",
				"tank",
				base,
			}, "\n"),
			want: storagePoolImportable,
		},
		{
			name: "missing pool",
			out: strings.Join([]string{
				"__K2_POOL_LIST__",
				"",
				"__K2_POOL_IMPORT__",
				"other",
				base,
			}, "\n"),
			want: storagePoolMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRemoteStorageInspection([]byte(tt.out), "tank")
			if err != nil {
				t.Fatal(err)
			}
			if got.PoolState != tt.want {
				t.Fatalf("PoolState = %q, want %q", got.PoolState, tt.want)
			}
		})
	}
}

func TestSplitMarkedSectionsEdges(t *testing.T) {
	got := splitMarkedSections("ignored\n__K2_POOL_LIST__\r\ntank ONLINE\n__K2_UNKNOWN__\nkept\n__K2_BYID__\nlinks\n")
	if strings.TrimSpace(got["__K2_POOL_LIST__"]) != "tank ONLINE\n__K2_UNKNOWN__\nkept" {
		t.Fatalf("pool list section = %q", got["__K2_POOL_LIST__"])
	}
	if strings.TrimSpace(got["__K2_BYID__"]) != "links" {
		t.Fatalf("byid section = %q", got["__K2_BYID__"])
	}
}

func TestStoragePoolScriptImportOnlyHasNoCreateOrWipe(t *testing.T) {
	got := storagePoolScript(storagePoolScriptInput{
		Pool:          "tank",
		ClusterName:   "v3",
		Compatibility: "openzfs-2.2-linux",
		CreateAllowed: false,
	})
	for _, want := range []string{
		`sudo zpool import "$pool"`,
		`pool $pool missing at execution time`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("script missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"zpool create", "wipefs"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("script contains %q:\n%s", forbidden, got)
		}
	}
}

func TestStorageInstallScriptQuotesHostsEntry(t *testing.T) {
	got := storageInstallScript("node name")
	if !strings.Contains(got, "grep -qxF '127.0.1.1 node name'") ||
		!strings.Contains(got, "echo '127.0.1.1 node name'") {
		t.Fatalf("script missing quoted hosts entry:\n%s", got)
	}
}
