package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/jayyao97/snap-commit/internal/git"
	"github.com/jayyao97/snap-commit/internal/storage"
)

// StoreCommand creates and stores a ghost commit snapshot
func StoreCommand(message string) error {
	// Resolve repository root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	repoRoot, err := git.ResolveRepoRoot(cwd)
	if err != nil {
		return err
	}

	// Create git client
	gitClient, err := git.NewGitClient(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to initialize git client: %w", err)
	}
	defer gitClient.Cleanup()

	// Capture untracked files before snapshot
	fmt.Println("Capturing current state...")
	untrackedFiles, untrackedDirs, err := gitClient.CaptureUntrackedFiles()
	if err != nil {
		return fmt.Errorf("failed to capture untracked files: %w", err)
	}

	// Create snapshot
	fmt.Println("Creating ghost commit...")
	if message == "" {
		message = "snap-commit snapshot"
	}
	commitSHA, parentSHA, err := gitClient.CreateSnapshot(message)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	now := time.Now()

	// Store snapshot metadata
	store, err := storage.NewSnapshotStore()
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	snapshot := storage.Snapshot{
		ID:                        storage.GenerateID(commitSHA, now),
		CommitSHA:                 commitSHA,
		ParentSHA:                 parentSHA,
		Message:                   message,
		Timestamp:                 now,
		RepoRoot:                  repoRoot,
		PreexistingUntrackedFiles: untrackedFiles,
		PreexistingUntrackedDirs:  untrackedDirs,
	}

	if err := store.AddSnapshot(snapshot); err != nil {
		return fmt.Errorf("failed to store snapshot: %w", err)
	}

	fmt.Printf("\n✓ Snapshot created successfully\n")
	fmt.Printf("  ID:        %s\n", snapshot.ID)
	fmt.Printf("  Commit:    %s\n", commitSHA[:12])
	fmt.Printf("  Message:   %s\n", message)
	fmt.Printf("  Timestamp: %s\n", snapshot.Timestamp.Format("2006-01-02 15:04:05"))

	return nil
}
