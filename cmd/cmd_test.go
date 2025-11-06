package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jayyao97/snap-commit/internal/storage"
)

func TestStoreAndRestoreCommands(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	repoDir := initGitRepo(t)
	if realDir, err := filepath.EvalSymlinks(repoDir); err == nil {
		repoDir = realDir
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}

	trackedPath := filepath.Join(repoDir, "tracked.txt")
	snapshotContent := []byte("snapshot state\n")
	if err := os.WriteFile(trackedPath, snapshotContent, 0644); err != nil {
		t.Fatalf("prepare tracked file: %v", err)
	}

	newTrackedPath := filepath.Join(repoDir, "newfile.txt")
	newTrackedContent := []byte("new tracked content\n")
	if err := os.WriteFile(newTrackedPath, newTrackedContent, 0644); err != nil {
		t.Fatalf("prepare new tracked file: %v", err)
	}

	preexistingUntracked := filepath.Join(repoDir, "preexisting.log")
	if err := os.WriteFile(preexistingUntracked, []byte("preexisting"), 0644); err != nil {
		t.Fatalf("write preexisting untracked file: %v", err)
	}

	if err := StoreCommand("test snapshot"); err != nil {
		t.Fatalf("StoreCommand error: %v", err)
	}

	store, err := storage.NewSnapshotStore()
	if err != nil {
		t.Fatalf("NewSnapshotStore error: %v", err)
	}

	snapshots, err := store.LoadSnapshots()
	if err != nil {
		t.Fatalf("LoadSnapshots error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	snapshot := snapshots[0]
	repoRoot := snapshot.RepoRoot
	if repoRoot != repoDir {
		t.Fatalf("expected snapshot RepoRoot %q, got %q", repoDir, repoRoot)
	}

	repoSnapshots, err := store.ListSnapshots(repoRoot)
	if err != nil {
		t.Fatalf("ListSnapshots by repo error: %v", err)
	}
	if len(repoSnapshots) != 1 || repoSnapshots[0].ID != snapshot.ID {
		t.Fatalf("expected repo-scoped list to contain snapshot %s", snapshot.ID)
	}
	if snapshot.Message != "test snapshot" {
		t.Fatalf("expected stored message 'test snapshot', got %q", snapshot.Message)
	}

	if err := os.WriteFile(trackedPath, []byte("corrupted\n"), 0644); err != nil {
		t.Fatalf("mutate tracked file: %v", err)
	}
	if err := os.Remove(newTrackedPath); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}

	newUntracked := filepath.Join(repoDir, "temp.bin")
	if err := os.WriteFile(newUntracked, []byte("temp"), 0644); err != nil {
		t.Fatalf("create new untracked file: %v", err)
	}

	newDir := filepath.Join(repoDir, "scratch")
	newDirFile := filepath.Join(newDir, "file.txt")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("create scratch dir: %v", err)
	}
	if err := os.WriteFile(newDirFile, []byte("scratch"), 0644); err != nil {
		t.Fatalf("write scratch file: %v", err)
	}

	if err := RestoreCommand("1", "", "", false); err != nil {
		t.Fatalf("RestoreCommand error: %v", err)
	}

	data, err := os.ReadFile(trackedPath)
	if err != nil {
		t.Fatalf("read tracked file after restore: %v", err)
	}
	if string(data) != string(snapshotContent) {
		t.Fatalf("tracked file mismatch after restore: %q", data)
	}

	if data, err := os.ReadFile(newTrackedPath); err != nil {
		t.Fatalf("expected new tracked file to be restored: %v", err)
	} else if string(data) != string(newTrackedContent) {
		t.Fatalf("new tracked file content mismatch: %q", data)
	}

	if _, err := os.Stat(newUntracked); !os.IsNotExist(err) {
		t.Fatalf("expected new untracked file removed, err=%v", err)
	}

	if _, err := os.Stat(newDirFile); !os.IsNotExist(err) {
		t.Fatalf("expected scratch file removed, err=%v", err)
	}
	if entries, err := os.ReadDir(newDir); err == nil && len(entries) != 0 {
		t.Fatalf("expected scratch directory empty, entries=%d", len(entries))
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking scratch directory: %v", err)
	}

	if _, err := os.Stat(preexistingUntracked); err != nil {
		t.Fatalf("expected preexisting untracked file to persist, err=%v", err)
	}

	snapshotsAfter, err := store.ListSnapshots(repoRoot)
	if err != nil {
		t.Fatalf("ListSnapshots after restore error: %v", err)
	}
	if len(snapshotsAfter) != 1 || snapshotsAfter[0].ID != snapshot.ID {
		t.Fatalf("expected snapshot to remain after non-destructive restore")
	}
}

func TestListCommandNoSnapshots(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	repoDir := initGitRepo(t)
	if realDir, err := filepath.EvalSymlinks(repoDir); err == nil {
		repoDir = realDir
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}

	if err := ListCommand(false); err != nil {
		t.Fatalf("ListCommand (current repo) error: %v", err)
	}

	if err := ListCommand(true); err != nil {
		t.Fatalf("ListCommand (all repos) error: %v", err)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")

	trackedPath := filepath.Join(repoDir, "tracked.txt")
	if err := os.WriteFile(trackedPath, []byte("initial\n"), 0644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}

	runGit(t, repoDir, "add", "tracked.txt")
	runGit(t, repoDir, "commit", "-m", "initial commit")

	return repoDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, output)
	}
}
