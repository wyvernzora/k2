package e2e

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type democraticCSIValues struct {
	CSIDriver      democraticCSIDriver      `yaml:"csiDriver"`
	StorageClasses []democraticStorageClass `yaml:"storageClasses"`
	Driver         democraticDriver         `yaml:"driver"`
	Node           democraticNodePlugin     `yaml:"node"`
}

type democraticCSIDriver struct {
	Name string `yaml:"name"`
}

type democraticStorageClass struct {
	Name                 string                       `yaml:"name"`
	DefaultClass         bool                         `yaml:"defaultClass"`
	ReclaimPolicy        string                       `yaml:"reclaimPolicy"`
	VolumeBindingMode    string                       `yaml:"volumeBindingMode"`
	AllowVolumeExpansion bool                         `yaml:"allowVolumeExpansion"`
	Parameters           map[string]string            `yaml:"parameters"`
	Secrets              map[string]map[string]string `yaml:"secrets"`
}

type democraticDriver struct {
	Config democraticDriverConfig `yaml:"config"`
}

type democraticDriverConfig struct {
	Driver string                `yaml:"driver"`
	SSH    democraticSSHConfig   `yaml:"sshConnection"`
	ZFS    democraticZFSConfig   `yaml:"zfs"`
	ISCSI  democraticISCSIConfig `yaml:"iscsi"`
}

type democraticSSHConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Username   string `yaml:"username"`
	PrivateKey string `yaml:"privateKey"`
}

type democraticZFSConfig struct {
	CLI                                map[string]bool `yaml:"cli"`
	DatasetParentName                  string          `yaml:"datasetParentName"`
	DetachedSnapshotsDatasetParentName string          `yaml:"detachedSnapshotsDatasetParentName"`
	ZvolEnableReservation              bool            `yaml:"zvolEnableReservation"`
	ZvolBlocksize                      string          `yaml:"zvolBlocksize"`
}

// Field placement mirrors democratic-csi's examples/zfs-generic-iscsi.yaml:
// targetPortal is a child of iscsi (NOT driver.config), and block sits under
// shareStrategyTargetCli. The driver reads iscsi.targetPortal to build the
// volume context; misplacing it leaves node staging with no portal at all.
type democraticISCSIConfig struct {
	ShareStrategy          string                        `yaml:"shareStrategy"`
	ShareStrategyTargetCli democraticShareStrategyTarget `yaml:"shareStrategyTargetCli"`
	Portal                 string                        `yaml:"targetPortal"`
}

type democraticShareStrategyTarget struct {
	SudoEnabled bool                   `yaml:"sudoEnabled"`
	Basename    string                 `yaml:"basename"`
	TPG         democraticTargetPortal `yaml:"tpg"`
	Block       democraticISCSIBlock   `yaml:"block"`
}

type democraticTargetPortal struct {
	Attributes map[string]int    `yaml:"attributes"`
	Auth       map[string]string `yaml:"auth"`
}

type democraticISCSIBlock struct {
	Attributes map[string]int `yaml:"attributes"`
}

type democraticNodePlugin struct {
	Driver democraticNodeDriver `yaml:"driver"`
}

type democraticNodeDriver struct {
	ISCSIDirHostPath     string `yaml:"iscsiDirHostPath"`
	ISCSIDirHostPathType string `yaml:"iscsiDirHostPathType"`
}

func democraticCSIValuesYAML(creds storageCredentials) ([]byte, error) {
	if creds.Portal == "" || creds.SSHHost == "" || creds.CSIPrivateKey == "" || creds.CHAPUsername == "" || creds.CHAPPassword == "" {
		return nil, fmt.Errorf("storage credentials are incomplete for democratic-csi values")
	}
	values := democraticCSIValues{
		CSIDriver: democraticCSIDriver{Name: "org.democratic-csi.iscsi"},
		StorageClasses: []democraticStorageClass{{
			Name:                 "zfs-iscsi",
			DefaultClass:         false,
			ReclaimPolicy:        "Delete",
			VolumeBindingMode:    "Immediate",
			AllowVolumeExpansion: true,
			Parameters:           map[string]string{"fsType": "ext4"},
			Secrets: map[string]map[string]string{
				"node-stage-secret": {
					"node-db.node.session.auth.authmethod":           "CHAP",
					"node-db.node.session.auth.username":             creds.CHAPUsername,
					"node-db.node.session.auth.password":             creds.CHAPPassword,
					"node-db.node.session.timeo.replacement_timeout": "180",
				},
			},
		}},
		Driver: democraticDriver{Config: democraticDriverConfig{
			Driver: "zfs-generic-iscsi",
			SSH: democraticSSHConfig{
				Host:       creds.SSHHost,
				Port:       creds.SSHPort,
				Username:   "csi",
				PrivateKey: creds.CSIPrivateKey,
			},
			ZFS: democraticZFSConfig{
				CLI:                                map[string]bool{"sudoEnabled": true},
				DatasetParentName:                  creds.DatasetParentName,
				DetachedSnapshotsDatasetParentName: creds.DetachedSnapshotsDatasetParentName,
				ZvolEnableReservation:              false,
				ZvolBlocksize:                      "16K",
			},
			ISCSI: democraticISCSIConfig{
				ShareStrategy: "targetCli",
				ShareStrategyTargetCli: democraticShareStrategyTarget{
					SudoEnabled: true,
					Basename:    creds.IQNBase,
					TPG: democraticTargetPortal{
						Attributes: map[string]int{
							"authentication":          1,
							"generate_node_acls":      1,
							"cache_dynamic_acls":      1,
							"demo_mode_write_protect": 0,
						},
						Auth: map[string]string{
							"userid":   creds.CHAPUsername,
							"password": creds.CHAPPassword,
						},
					},
					Block: democraticISCSIBlock{Attributes: map[string]int{"emulate_tpu": 1}},
				},
				Portal: creds.Portal,
			},
		}},
		Node: democraticNodePlugin{Driver: democraticNodeDriver{
			ISCSIDirHostPath:     "/etc/iscsi",
			ISCSIDirHostPathType: "DirectoryOrCreate",
		}},
	}
	return yaml.Marshal(values)
}
