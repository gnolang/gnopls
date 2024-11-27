package tools

import (
	"fmt"
	"os/exec"
	"strings"
)

func Format(file string) ([]byte, error) {
	cmd := exec.Command("gno", "fmt", file)
	bz, err := cmd.CombinedOutput()
	if err != nil {
		return bz, fmt.Errorf("running '%s': %w: %s", strings.Join(cmd.Args, " "), err, string(bz))
	}
	return bz, nil
}
