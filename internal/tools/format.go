package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Format(source []byte) ([]byte, error) {
	// gno fmt accepts only files, so we need to write source in a temp file.
	tmpDir, err := os.MkdirTemp("", "gnopls-fmt")
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	tmpFile := filepath.Join(tmpDir, "file.gno")
	err = os.WriteFile(tmpFile, source, 0o600)
	if err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}
	defer os.Remove(tmpDir)
	cmd := exec.Command("gno", "fmt", tmpFile)
	var stdin, stderr bytes.Buffer
	cmd.Stdout = &stdin
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("format: running '%s': %w: %s", strings.Join(cmd.Args, " "), err, stderr.String())
	}
	return stdin.Bytes(), nil
}
