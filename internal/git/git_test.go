package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitClientSnapshotWorkflow(t *testing.T) {
	repoDir := t.TempDir()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")

	trackedFile := filepath.Join(repoDir, "tracked.txt")
	if err := os.WriteFile(trackedFile, []byte("initial\n"), 0644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}

	runGit(t, repoDir, "add", "tracked.txt")
	runGit(t, repoDir, "commit", "-m", "initial commit")

	client, err := NewGitClient(repoDir)
	if err != nil {
		t.Fatalf("NewGitClient error: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Cleanup(); err != nil {
			t.Fatalf("cleanup error: %v", err)
		}
	})

	headBefore, err := client.GetHEAD()
	if err != nil {
		t.Fatalf("GetHEAD error: %v", err)
	}

	if err := os.WriteFile(trackedFile, []byte("snapshot version\n"), 0644); err != nil {
		t.Fatalf("prepare tracked file: %v", err)
	}

	newTracked := filepath.Join(repoDir, "newfile.txt")
	if err := os.WriteFile(newTracked, []byte("new file content\n"), 0644); err != nil {
		t.Fatalf("write new tracked file: %v", err)
	}

	preexistingUntracked := filepath.Join(repoDir, "preexisting.log")
	if err := os.WriteFile(preexistingUntracked, []byte("keep me\n"), 0644); err != nil {
		t.Fatalf("write preexisting untracked file: %v", err)
	}

	preFiles, preDirs, err := client.CaptureUntrackedFiles()
	if err != nil {
		t.Fatalf("CaptureUntrackedFiles error: %v", err)
	}

	commitSHA, parentSHA, err := client.CreateSnapshot("test snapshot")
	if err != nil {
		t.Fatalf("CreateSnapshot error: %v", err)
	}

	if len(commitSHA) != 40 {
		t.Fatalf("expected commit SHA length 40, got %d", len(commitSHA))
	}
	if parentSHA != headBefore {
		t.Fatalf("expected parent %s, got %s", headBefore, parentSHA)
	}

	// Mutate repository after snapshot
	if err := os.WriteFile(trackedFile, []byte("broken state\n"), 0644); err != nil {
		t.Fatalf("mutate tracked file: %v", err)
	}
	if err := os.Remove(newTracked); err != nil {
		t.Fatalf("remove new tracked file: %v", err)
	}

	newUntracked := filepath.Join(repoDir, "temp.bin")
	if err := os.WriteFile(newUntracked, []byte("temporary data"), 0644); err != nil {
		t.Fatalf("write new untracked file: %v", err)
	}

	newDir := filepath.Join(repoDir, "scratch")
	newDirFile := filepath.Join(newDir, "file.txt")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("create scratch dir: %v", err)
	}
	if err := os.WriteFile(newDirFile, []byte("scratch"), 0644); err != nil {
		t.Fatalf("write scratch file: %v", err)
	}

	if err := client.RestoreSnapshot(commitSHA); err != nil {
		t.Fatalf("RestoreSnapshot error: %v", err)
	}

	if data, err := os.ReadFile(trackedFile); err != nil {
		t.Fatalf("read tracked after restore: %v", err)
	} else if string(data) != "snapshot version\n" {
		t.Fatalf("tracked file restored content mismatch: %q", data)
	}

	if _, err := os.Stat(newTracked); err != nil {
		t.Fatalf("expected new tracked file to be restored: %v", err)
	}

	if err := client.RemoveUntrackedFiles(preFiles, preDirs); err != nil {
		t.Fatalf("RemoveUntrackedFiles error: %v", err)
	}

	if _, err := os.Stat(newUntracked); !os.IsNotExist(err) {
		t.Fatalf("expected new untracked file removed, err=%v", err)
	}

	if _, err := os.Stat(newDirFile); !os.IsNotExist(err) {
		t.Fatalf("expected new untracked file in directory removed, err=%v", err)
	}
	if entries, err := os.ReadDir(newDir); err == nil && len(entries) != 0 {
		t.Fatalf("expected scratch directory empty after cleanup, entries=%d", len(entries))
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking scratch directory: %v", err)
	}

	if _, err := os.Stat(preexistingUntracked); err != nil {
		t.Fatalf("expected preexisting untracked file to remain, err=%v", err)
	}
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
