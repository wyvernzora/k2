package e2e

func (c *e2eListCmd) Run(rcx *Runtime) error {
	scenarios, err := listE2EScenarios(rcx.RepoRoot)
	if err != nil {
		return err
	}
	rows := make([][]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		rows = append(rows, []string{scenario.Name, scenario.Description})
	}
	currentReporter().Table([]string{"SCENARIO", "DESCRIPTION"}, rows)
	return nil
}
