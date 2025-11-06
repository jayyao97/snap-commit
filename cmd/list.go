package cmd

import (
	"fmt"
	"os"

	"github.com/jayyao97/snap-commit/internal/git"
	"github.com/jayyao97/snap-commit/internal/storage"
	"github.com/jayyao97/snap-commit/internal/tui"
)

// ListCommand lists all snapshots for the current repository
func ListCommand(allRepos bool) error {
	var (
		repoRoot string
		err      error
	)

	if !allRepos {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		repoRoot, err = git.ResolveRepoRoot(cwd)
		if err != nil {
			return err
		}
	}

	// Load snapshots
	store, err := storage.NewSnapshotStore()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	var snapshots []storage.Snapshot
	if allRepos {
		snapshots, err = store.LoadSnapshots()
		if err != nil {
			return fmt.Errorf("failed to load snapshots: %w", err)
		}
	} else {
		snapshots, err = store.ListSnapshots(repoRoot)
		if err != nil {
			return fmt.Errorf("failed to list snapshots: %w", err)
		}
	}

	if len(snapshots) == 0 {
		if allRepos {
			fmt.Println("No snapshots found")
		} else {
			fmt.Println("No snapshots found for this repository")
		}
		return nil
	}

	title := "Snapshots (current repo)"
	if allRepos {
		title = "Snapshots (all repositories)"
	}

	deleteHandler := func(snapshot *storage.Snapshot) error {
		return store.DeleteSnapshot(snapshot.RepoRoot, snapshot.ID)
	}

	if err := tui.BrowseSnapshots(snapshots, title, allRepos, deleteHandler); err != nil {
		return fmt.Errorf("interactive list failed: %w", err)
	}

	return nil
}
