package provision

func (c *serverCmd) Run(rcx *Runtime) error {
	return provisionJoinNode(rcx, nodeRoleServer, c.commonJoinFlags, c.commonRemoteFlags)
}
