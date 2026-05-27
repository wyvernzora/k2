package toolcli

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

func (c *imagePlanCmd) Run(ctx *runContext, parent *imageCmd) error {
	planner, err := loadImagePlanner(parent.imageGlobals())
	if err != nil {
		return err
	}

	plans, err := imagePlansForSelection(planner, c.All, c.Target, "image plan")
	if err != nil {
		return err
	}
	if len(plans) == 1 && !c.All {
		return writeImageOutput(c.Format, plans[0])
	}
	return writeImageOutput(c.Format, imagePlansOutput{Targets: plans})
}

func writeImageOutput(format string, value any) error {
	switch format {
	case "json":
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
	case "yaml":
		encoded, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		fmt.Print(string(encoded))
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	return nil
}
