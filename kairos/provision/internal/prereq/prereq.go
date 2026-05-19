package prereq

import (
	"fmt"
	"os/exec"
	"strings"
)

func Require(commands ...string) error {
	var missing []string
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			missing = append(missing, command)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required command(s): %s", strings.Join(missing, ", "))
	}
	return nil
}
