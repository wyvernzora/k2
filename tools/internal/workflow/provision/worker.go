package provision

func (c *workerCmd) Run(rcx *Runtime) error {
	return provisionJoinNode(rcx, nodeRoleWorker, c.commonJoinFlags, c.commonRemoteFlags)
}
