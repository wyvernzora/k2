package provision

import "context"

type StorageInputs struct {
	ClusterTarget     string
	ClusterName       string
	NodeName          string
	Pool              string
	PoolVDev          []string
	OperatorFiles     []string
	IQNBase           string
	PoolCompatibility string
	TestVM            string
	Host              string
	SSHPort           int
	SSHUser           string
	Identity          string
	Yes               bool
	NoPasswordPrompt  bool
}

func RunStorage(ctx context.Context, runtime *Runtime, in StorageInputs) error {
	cmd := storageCmd{
		commonStorageFlags: commonStorageFlags{
			ClusterTarget: in.ClusterTarget, ClusterName: in.ClusterName, NodeName: in.NodeName,
			Pool: in.Pool, PoolVDev: in.PoolVDev, OperatorFiles: in.OperatorFiles,
			IQNBase: in.IQNBase, PoolCompatibility: in.PoolCompatibility,
		},
		TestVM: in.TestVM, Host: in.Host, SSHPort: in.SSHPort, SSHUser: in.SSHUser,
		Identity: in.Identity, Yes: in.Yes, noPasswordPrompt: in.NoPasswordPrompt,
	}
	_, err := runStorageProvision(ctx, runtime, &cmd)
	return err
}
