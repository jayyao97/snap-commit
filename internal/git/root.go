package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveRepoRoot returns the top-level git directory for the provided path.
func ResolveRepoRoot(startDir string) (string, error) {
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		startDir = cwd
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = startDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to resolve git root from %s: %w\nOutput: %s",
			startDir, err, strings.TrimSpace(string(output)))
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		return "", fmt.Errorf("git root not found from %s", startDir)
	}

	return filepath.Clean(root), nil
}
