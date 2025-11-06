package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Snapshot represents a stored ghost commit
type Snapshot struct {
	ID                        string    `json:"id"`                          // Unique identifier
	CommitSHA                 string    `json:"commit_sha"`                  // Ghost commit SHA
	ParentSHA                 string    `json:"parent_sha"`                  // Parent commit SHA (HEAD at snapshot time)
	Message                   string    `json:"message"`                     // Snapshot message
	Timestamp                 time.Time `json:"timestamp"`                   // When snapshot was created
	RepoRoot                  string    `json:"repo_root"`                   // Repository root path
	PreexistingUntrackedFiles []string  `json:"preexisting_untracked_files"` // Untracked files before snapshot
	PreexistingUntrackedDirs  []string  `json:"preexisting_untracked_dirs"`  // Untracked dirs before snapshot
}

// SnapshotStore manages snapshot persistence
type SnapshotStore struct {
	StorageDir   string
	repoFilename string
}

// NewSnapshotStore creates a new snapshot store
func NewSnapshotStore() (*SnapshotStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storageDir := filepath.Join(homeDir, ".snap-commit")

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &SnapshotStore{
		StorageDir:   storageDir,
		repoFilename: "snapshots.json",
	}, nil
}

// LoadSnapshots loads all snapshots from all repositories
func (s *SnapshotStore) LoadSnapshots() ([]Snapshot, error) {
	entries, err := os.ReadDir(s.StorageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	var all []Snapshot
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(s.StorageDir, entry.Name(), s.repoFilename)
		snaps, err := s.readSnapshotsFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		all = append(all, snaps...)
	}

	return all, nil
}

// AddSnapshot adds a new snapshot scoped to its repository
func (s *SnapshotStore) AddSnapshot(snapshot Snapshot) error {
	snapshots, err := s.loadRepoSnapshots(snapshot.RepoRoot)
	if err != nil {
		return err
	}

	snapshots = append(snapshots, snapshot)
	return s.saveRepoSnapshots(snapshot.RepoRoot, snapshots)
}

// GetSnapshot retrieves a snapshot by ID or repo-scoped index (1-based for user convenience)
func (s *SnapshotStore) GetSnapshot(identifier, repoRoot string) (*Snapshot, error) {
	snapshots, err := s.loadRepoSnapshots(repoRoot)
	if err != nil {
		return nil, err
	}

	// Try to find by ID first
	for i := range snapshots {
		if snapshots[i].ID == identifier {
			return &snapshots[i], nil
		}
	}

	// Try to parse as index (1-based) scoped to the repository
	if index, err := strconv.Atoi(identifier); err == nil {
		if len(snapshots) == 0 {
			return nil, fmt.Errorf("no snapshots found for repository: %s", repoRoot)
		}
		if index < 1 || index > len(snapshots) {
			return nil, fmt.Errorf(
				"snapshot index %d out of range for repository (1-%d)",
				index,
				len(snapshots),
			)
		}
		return &snapshots[index-1], nil
	}

	return nil, fmt.Errorf("snapshot not found: %s", identifier)
}

// GetSnapshotByCommit finds a snapshot for the repo by commit SHA (prefix allowed).
func (s *SnapshotStore) GetSnapshotByCommit(repoRoot, commit string) (*Snapshot, error) {
	snapshots, err := s.ListSnapshots(repoRoot)
	if err != nil {
		return nil, err
	}

	var matches []*Snapshot
	for i := range snapshots {
		if strings.HasPrefix(snapshots[i].CommitSHA, commit) {
			matches = append(matches, &snapshots[i])
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no snapshot matching commit %s", commit)
	}
	// More than one match is ambiguous
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple snapshots match commit prefix %s", commit)
	}

	return matches[0], nil
}

// ListSnapshots returns all snapshots for a specific repository
func (s *SnapshotStore) ListSnapshots(repoRoot string) ([]Snapshot, error) {
	return s.loadRepoSnapshots(repoRoot)
}

// DeleteSnapshot removes a snapshot by ID within the repo
func (s *SnapshotStore) DeleteSnapshot(repoRoot, id string) error {
	snapshots, err := s.loadRepoSnapshots(repoRoot)
	if err != nil {
		return err
	}

	found := false
	filtered := make([]Snapshot, 0, len(snapshots))
	for _, snap := range snapshots {
		if snap.ID != id {
			filtered = append(filtered, snap)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("snapshot not found: %s", id)
	}

	return s.saveRepoSnapshots(repoRoot, filtered)
}

const snapshotIDCommitPrefix = 12

// GenerateID builds a human-readable snapshot ID combining the snapshot commit
// prefix and a base-36 encoded timestamp for chronological sorting.
func GenerateID(commitSHA string, ts time.Time) string {
	prefixLen := snapshotIDCommitPrefix
	if len(commitSHA) < prefixLen {
		prefixLen = len(commitSHA)
	}
	if prefixLen == 0 {
		commitSHA = "unknown"
		prefixLen = len(commitSHA)
	}

	encodedTime := strconv.FormatInt(ts.UnixNano(), 36)
	return commitSHA[:prefixLen] + encodedTime
}

// repoKey returns a stable directory name for a repository root.
func repoKey(repoRoot string) string {
	sum := sha1.Sum([]byte(repoRoot))
	return hex.EncodeToString(sum[:])
}

func (s *SnapshotStore) repoDir(repoRoot string) string {
	return filepath.Join(s.StorageDir, repoKey(repoRoot))
}

func (s *SnapshotStore) repoFile(repoRoot string) string {
	return filepath.Join(s.repoDir(repoRoot), s.repoFilename)
}

func (s *SnapshotStore) loadRepoSnapshots(repoRoot string) ([]Snapshot, error) {
	path := s.repoFile(repoRoot)
	return s.readSnapshotsFile(path)
}

func (s *SnapshotStore) saveRepoSnapshots(repoRoot string, snapshots []Snapshot) error {
	return s.writeRepoSnapshots(repoRoot, snapshots)
}

func (s *SnapshotStore) writeRepoSnapshots(repoRoot string, snapshots []Snapshot) error {
	dir := s.repoDir(repoRoot)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create repo storage dir: %w", err)
	}

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshots: %w", err)
	}

	if err := os.WriteFile(s.repoFile(repoRoot), data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshots file: %w", err)
	}

	return nil
}

func (s *SnapshotStore) readSnapshotsFile(path string) ([]Snapshot, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return []Snapshot{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to stat snapshots file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshots file: %w", err)
	}

	var snapshots []Snapshot
	if len(data) == 0 {
		return []Snapshot{}, nil
	}

	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, fmt.Errorf("failed to parse snapshots file: %w", err)
	}

	return snapshots, nil
}
