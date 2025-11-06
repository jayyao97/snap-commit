package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/jayyao97/snap-commit/internal/git"
	"github.com/jayyao97/snap-commit/internal/storage"
	"github.com/jayyao97/snap-commit/internal/tui"
)

// RestoreCommand restores the working tree to a snapshot
func RestoreCommand(identifier, idFlag, commitFlag string, deleteSnapshot bool) error {
	// Resolve repository root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	repoRoot, err := git.ResolveRepoRoot(cwd)
	if err != nil {
		return err
	}

	// Load snapshot
	store, err := storage.NewSnapshotStore()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	var snapshot *storage.Snapshot
	switch {
	case idFlag != "":
		snapshot, err = store.GetSnapshot(idFlag, repoRoot)
		if err != nil {
			return fmt.Errorf("failed to get snapshot by ID: %w", err)
		}
	case commitFlag != "":
		snapshot, err = store.GetSnapshotByCommit(repoRoot, commitFlag)
		if err != nil {
			return fmt.Errorf("failed to get snapshot by commit: %w", err)
		}
	case identifier != "":
		snapshot, err = store.GetSnapshot(identifier, repoRoot)
		if err != nil {
			return fmt.Errorf("failed to get snapshot: %w", err)
		}
	default:
		snapshots, err := store.ListSnapshots(repoRoot)
		if err != nil {
			return fmt.Errorf("failed to list snapshots: %w", err)
		}
		if len(snapshots) == 0 {
			return fmt.Errorf("no snapshots found for this repository")
		}

		snapshot, err = tui.SelectSnapshot(snapshots, "Select snapshot to restore", false)
		if errors.Is(err, tui.ErrAborted) {
			fmt.Println("Restore cancelled")
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to select snapshot: %w", err)
		}
	}

	// Verify snapshot is for current repository
	if snapshot.RepoRoot != repoRoot {
		return fmt.Errorf("snapshot is for different repository: %s", snapshot.RepoRoot)
	}

	// Create git client
	gitClient, err := git.NewGitClient(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to initialize git client: %w", err)
	}
	defer gitClient.Cleanup()

	fmt.Printf("Restoring snapshot...\n")
	fmt.Printf("  ID:      %s\n", snapshot.ID)
	fmt.Printf("  Commit:  %s\n", snapshot.CommitSHA[:12])
	fmt.Printf("  Message: %s\n", snapshot.Message)
	fmt.Println()

	// Verify commit object still exists before attempting restore
	if !gitClient.ObjectExists(snapshot.CommitSHA) {
		return fmt.Errorf(
			"snapshot commit %s no longer exists (likely pruned by git)",
			snapshot.CommitSHA,
		)
	}

	// Restore the snapshot
	fmt.Println("Restoring working tree...")
	if err := gitClient.RestoreSnapshot(snapshot.CommitSHA); err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	// Remove untracked files created after snapshot
	fmt.Println("Cleaning up untracked files...")
	if err := gitClient.RemoveUntrackedFiles(
		snapshot.PreexistingUntrackedFiles,
		snapshot.PreexistingUntrackedDirs,
	); err != nil {
		return fmt.Errorf("failed to clean untracked files: %w", err)
	}

	// Delete snapshot if requested
	if deleteSnapshot {
		if err := store.DeleteSnapshot(repoRoot, snapshot.ID); err != nil {
			return fmt.Errorf("failed to delete snapshot: %w", err)
		}
		fmt.Println("\n✓ Snapshot restored and deleted")
	} else {
		fmt.Println("\n✓ Snapshot restored successfully")
	}

	return nil
}
