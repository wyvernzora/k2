package provision

import "context"

type WorkerInputs struct {
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

func RunWorker(ctx context.Context, runtime *Runtime, in WorkerInputs) error {
	return runJoinProvision(ctx, runtime, nodeRoleWorker,
		commonJoinFlags{
			ClusterTarget: in.ClusterTarget, ClusterName: in.ClusterName, NodeName: in.NodeName, OperatorFiles: in.OperatorFiles,
		},
		commonRemoteFlags{
			TestVM: in.TestVM, SSHUser: in.SSHUser, Identity: in.Identity, Yes: in.Yes, noPasswordPrompt: in.NoPasswordPrompt,
		},
	)
}
