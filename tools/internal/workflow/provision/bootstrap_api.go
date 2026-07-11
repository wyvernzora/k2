package provision

import "context"

type BootstrapInputs struct {
	ClusterTarget    string
	ClusterName      string
	NodeName         string
	OperatorFiles    []string
	TestVM           string
	SSHUser          string
	Identity         string
	Yes              bool
	NoPasswordPrompt bool
}

func RunBootstrap(ctx context.Context, runtime *Runtime, in BootstrapInputs) error {
	cmd := bootstrapCmd{
		commonBootstrapFlags: commonBootstrapFlags{
			ClusterTarget: in.ClusterTarget, ClusterName: in.ClusterName, NodeName: in.NodeName, OperatorFiles: in.OperatorFiles,
		},
		TestVM: in.TestVM, SSHUser: in.SSHUser, Identity: in.Identity, Yes: in.Yes, noPasswordPrompt: in.NoPasswordPrompt,
	}
	return runBootstrapProvision(ctx, runtime, &cmd)
}
