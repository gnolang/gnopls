package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func Format(file string) ([]byte, error) {
	cmd := exec.Command("gno", "fmt", file)
	var stdin, stderr bytes.Buffer
	cmd.Stdout = &stdin
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("running '%s': %w: %s", strings.Join(cmd.Args, " "), err, stderr.String())
	}
	return stdin.Bytes(), nil
}
