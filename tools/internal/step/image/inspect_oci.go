package image

import (
	"fmt"
	"io"

	"github.com/wyvernzora/k2/tools/internal/image/plan"
)

func (i Inspector) OCI(resolved plan.Plan, imageOverride string) error {
	i = i.withDefaults()
	if i.Stdout == nil {
		i.Stdout = io.Discard
	}
	image := resolved.Image
	if imageOverride != "" {
		image = imageOverride
	}
	for _, absent := range resolved.Inspection.OCI.Absent {
		if err := i.runContainerShell(image, "[ ! -e "+shellQuote(absent.Path)+" ]"); err != nil {
			return fmt.Errorf("OCI path %s should be absent: %w", absent.Path, err)
		}
		fmt.Fprintf(i.Stdout, "OCI absent %s: OK\n", absent.Path)
	}
	for _, command := range resolved.Inspection.OCI.Commands {
		if err := i.runContainerShell(image, "command -v -- "+shellQuote(command.Name)+" >/dev/null"); err != nil {
			return fmt.Errorf("OCI command %s not found: %w", command.Name, err)
		}
		fmt.Fprintf(i.Stdout, "OCI command %s: OK\n", command.Name)
	}
	for _, file := range resolved.Inspection.OCI.Files {
		data, err := i.readContainerFile(image, file.Path)
		if err != nil {
			return err
		}
		if err := validateFileExpectation(data, file, "OCI file "); err != nil {
			return err
		}
		fmt.Fprintf(i.Stdout, "OCI file %s: OK\n", file.Path)
	}
	fmt.Fprintf(i.Stdout, "OCI inspection passed for %s\n", resolved.Target)
	return nil
}
