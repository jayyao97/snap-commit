package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitClient handles git operations using temporary index
type GitClient struct {
	RepoRoot string
	TempDir  string
}

// NewGitClient creates a new git client for the given repository
func NewGitClient(repoRoot string) (*GitClient, error) {
	root, err := ResolveRepoRoot(repoRoot)
	if err != nil {
		return nil, err
	}

	// Create temporary directory for git index
	tempDir, err := os.MkdirTemp("", "snap-commit-git-index-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	return &GitClient{
		RepoRoot: root,
		TempDir:  tempDir,
	}, nil
}

// Cleanup removes temporary files
func (g *GitClient) Cleanup() error {
	if g.TempDir != "" {
		return os.RemoveAll(g.TempDir)
	}
	return nil
}

// runGit executes a git command with the temporary index
func (g *GitClient) runGit(args []string, useTempIndex bool) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoRoot

	if useTempIndex {
		indexPath := filepath.Join(g.TempDir, "index")
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_INDEX_FILE=%s", indexPath))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\nOutput: %s", strings.Join(args, " "), err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// GetHEAD returns the current HEAD commit SHA, or empty string if no commits
func (g *GitClient) GetHEAD() (string, error) {
	output, err := g.runGit([]string{"rev-parse", "HEAD"}, false)
	if err != nil {
		// Check if it's because there are no commits yet
		if strings.Contains(err.Error(), "unknown revision") || strings.Contains(err.Error(), "ambiguous argument 'HEAD'") {
			return "", nil
		}
		return "", err
	}
	return output, nil
}

// CaptureUntrackedFiles returns list of untracked files and directories
func (g *GitClient) CaptureUntrackedFiles() (files []string, dirs []string, err error) {
	output, err := g.runGit([]string{
		"status",
		"--porcelain=2",
		"-z",
		"--ignored=matching",
		"--untracked-files=all",
	}, false)
	if err != nil {
		return nil, nil, err
	}

	if output == "" {
		return []string{}, []string{}, nil
	}

	// Parse null-terminated output
	entries := strings.Split(output, "\x00")
	for _, entry := range entries {
		if entry == "" {
			continue
		}

		// porcelain v2 format: "? <path>" for untracked
		// "! <path>" for ignored
		fields := strings.Fields(entry)
		if len(fields) < 2 {
			continue
		}

		if fields[0] == "?" || fields[0] == "!" {
			path := strings.Join(fields[1:], " ")
			// Check if it's a directory
			fullPath := filepath.Join(g.RepoRoot, path)
			info, err := os.Stat(fullPath)
			if err == nil && info.IsDir() {
				dirs = append(dirs, path)
			} else {
				files = append(files, path)
			}
		}
	}

	return files, dirs, nil
}

// CreateSnapshot creates a ghost commit and returns its SHA
func (g *GitClient) CreateSnapshot(message string) (commitSHA string, parentSHA string, err error) {
	// Get current HEAD
	parent, err := g.GetHEAD()
	if err != nil {
		return "", "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// If HEAD exists, populate temp index with it
	if parent != "" {
		_, err = g.runGit([]string{"read-tree", parent}, true)
		if err != nil {
			return "", "", fmt.Errorf("failed to read-tree: %w", err)
		}
	}

	// Stage all changes
	_, err = g.runGit([]string{"add", "--all"}, true)
	if err != nil {
		return "", "", fmt.Errorf("failed to add files: %w", err)
	}

	// Create tree object
	treeSHA, err := g.runGit([]string{"write-tree"}, true)
	if err != nil {
		return "", "", fmt.Errorf("failed to write-tree: %w", err)
	}

	// Create commit object
	commitArgs := []string{"commit-tree", treeSHA}
	if parent != "" {
		commitArgs = append(commitArgs, "-p", parent)
	}
	commitArgs = append(commitArgs, "-m", message)

	cmd := exec.Command("git", commitArgs...)
	cmd.Dir = g.RepoRoot
	indexPath := filepath.Join(g.TempDir, "index")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GIT_INDEX_FILE=%s", indexPath),
		"GIT_AUTHOR_NAME=Snap Commit",
		"GIT_AUTHOR_EMAIL=snapshot@snap-commit.local",
		"GIT_COMMITTER_NAME=Snap Commit",
		"GIT_COMMITTER_EMAIL=snapshot@snap-commit.local",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("failed to commit-tree: %w\nOutput: %s", err, string(output))
	}

	commitSHA = strings.TrimSpace(string(output))
	return commitSHA, parent, nil
}

// RestoreSnapshot restores working tree and index to a specific commit
func (g *GitClient) RestoreSnapshot(commitSHA string) error {
	// Restore both working tree and index
	_, err := g.runGit([]string{
		"restore",
		"--source=" + commitSHA,
		"--worktree",
		"--staged",
		".",
	}, false)
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	return nil
}

// RemoveUntrackedFiles removes files/dirs that were created after the snapshot
func (g *GitClient) RemoveUntrackedFiles(preexistingFiles, preexistingDirs []string) error {
	// Get current untracked files
	currentFiles, currentDirs, err := g.CaptureUntrackedFiles()
	if err != nil {
		return fmt.Errorf("failed to capture current untracked: %w", err)
	}

	// Convert to maps for easy lookup
	preexistingFileMap := make(map[string]bool)
	for _, f := range preexistingFiles {
		preexistingFileMap[f] = true
	}
	preexistingDirMap := make(map[string]bool)
	for _, d := range preexistingDirs {
		preexistingDirMap[d] = true
	}

	// Remove files that didn't exist before
	for _, f := range currentFiles {
		if !preexistingFileMap[f] {
			fullPath := filepath.Join(g.RepoRoot, f)
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove file %s: %w", f, err)
			}
		}
	}

	// Remove directories that didn't exist before (in reverse order to handle nested dirs)
	for i := len(currentDirs) - 1; i >= 0; i-- {
		d := currentDirs[i]
		if !preexistingDirMap[d] {
			fullPath := filepath.Join(g.RepoRoot, d)
			if err := os.RemoveAll(fullPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove directory %s: %w", d, err)
			}
		}
	}

	return nil
}

// ObjectExists checks whether a git object is reachable in the object database.
func (g *GitClient) ObjectExists(objectID string) bool {
	_, err := g.runGit([]string{"cat-file", "-e", objectID}, false)
	return err == nil
}
