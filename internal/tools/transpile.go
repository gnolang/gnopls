package tools

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Transpile a Gno package: gno transpile <dir>.
func Transpile(rootDir string) ([]byte, error) {
	cmd := exec.Command("gno", "transpile", "-skip-imports", filepath.Join(rootDir))
	bz, err := cmd.CombinedOutput()
	if err != nil {
		return bz, fmt.Errorf("running '%s': %w: %s", strings.Join(cmd.Args, " "), err, string(bz))
	}
	return bz, nil
}
