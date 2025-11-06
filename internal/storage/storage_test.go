package storage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGenerateIDFormat(t *testing.T) {
	ts := time.Unix(1_700_000_000, 123456789)
	commit := "0123456789abcdef0123456789abcdef01234567"

	id := GenerateID(commit, ts)

	if len(id) <= snapshotIDCommitPrefix {
		t.Fatalf("expected id length greater than prefix, got %q", id)
	}

	expectedPrefix := commit[:snapshotIDCommitPrefix]
	if !strings.HasPrefix(id, expectedPrefix) {
		t.Fatalf("expected id to start with %q, got %q", expectedPrefix, id)
	}

	suffix := id[len(expectedPrefix):]
	parsed, err := strconv.ParseInt(suffix, 36, 64)
	if err != nil {
		t.Fatalf("failed to parse timestamp suffix %q: %v", suffix, err)
	}
	if parsed != ts.UnixNano() {
		t.Fatalf("expected timestamp %d, got %d", ts.UnixNano(), parsed)
	}

	shortCommit := "deadbeef"
	shortID := GenerateID(shortCommit, ts)
	if !strings.HasPrefix(shortID, shortCommit) {
		t.Fatalf("expected short commit prefix %q in id %q", shortCommit, shortID)
	}

	emptyID := GenerateID("", ts)
	if !strings.HasPrefix(emptyID, "unknown") {
		t.Fatalf("expected fallback prefix 'unknown' in id %q", emptyID)
	}
}

func TestSnapshotStoreCRUD(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("USERPROFILE", tempHome)

	store, err := NewSnapshotStore()
	if err != nil {
		t.Fatalf("NewSnapshotStore() error = %v", err)
	}

	repoRoot := filepath.Join(tempHome, "repos", "demo")

	ts1 := time.Unix(1_700_000_000, 0)
	commit1 := strings.Repeat("a", 40)
	parent1 := strings.Repeat("b", 40)
	snap1 := Snapshot{
		ID:        GenerateID(commit1, ts1),
		CommitSHA: commit1,
		ParentSHA: parent1,
		Message:   "first snapshot",
		Timestamp: ts1,
		RepoRoot:  repoRoot,
	}

	if err := store.AddSnapshot(snap1); err != nil {
		t.Fatalf("AddSnapshot snap1 error = %v", err)
	}

	ts2 := ts1.Add(time.Minute)
	commit2 := strings.Repeat("c", 40)
	parent2 := commit1
	snap2 := Snapshot{
		ID:        GenerateID(commit2, ts2),
		CommitSHA: commit2,
		ParentSHA: parent2,
		Message:   "second snapshot",
		Timestamp: ts2,
		RepoRoot:  repoRoot,
	}

	if err := store.AddSnapshot(snap2); err != nil {
		t.Fatalf("AddSnapshot snap2 error = %v", err)
	}

	list, err := store.ListSnapshots(repoRoot)
	if err != nil {
		t.Fatalf("ListSnapshots error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(list))
	}
	if list[0].ID != snap1.ID || list[1].ID != snap2.ID {
		t.Fatalf("expected snapshots in insertion order [%s, %s], got [%s, %s]",
			snap1.ID, snap2.ID, list[0].ID, list[1].ID)
	}

	gotByID, err := store.GetSnapshot(snap1.ID, repoRoot)
	if err != nil {
		t.Fatalf("GetSnapshot by ID error = %v", err)
	}
	if gotByID.ID != snap1.ID || !gotByID.Timestamp.Equal(snap1.Timestamp) {
		t.Fatalf("GetSnapshot by ID mismatch: %+v", gotByID)
	}

	gotByIndex, err := store.GetSnapshot("2", repoRoot)
	if err != nil {
		t.Fatalf("GetSnapshot by index error = %v", err)
	}
	if gotByIndex.ID != snap2.ID {
		t.Fatalf("expected index 2 to return %s, got %s", snap2.ID, gotByIndex.ID)
	}

	commitPrefix := commit2[:8]
	gotByCommit, err := store.GetSnapshotByCommit(repoRoot, commitPrefix)
	if err != nil {
		t.Fatalf("GetSnapshotByCommit error = %v", err)
	}
	if gotByCommit.ID != snap2.ID {
		t.Fatalf("expected commit prefix lookup to return %s, got %s", snap2.ID, gotByCommit.ID)
	}

	if err := store.DeleteSnapshot(repoRoot, snap1.ID); err != nil {
		t.Fatalf("DeleteSnapshot error = %v", err)
	}

	listAfterDelete, err := store.ListSnapshots(repoRoot)
	if err != nil {
		t.Fatalf("ListSnapshots after delete error = %v", err)
	}
	if len(listAfterDelete) != 1 {
		t.Fatalf("expected 1 snapshot after delete, got %d", len(listAfterDelete))
	}
	if listAfterDelete[0].ID != snap2.ID {
		t.Fatalf("expected remaining snapshot %s, got %s", snap2.ID, listAfterDelete[0].ID)
	}

	// Ensure snapshots.json was written to the custom HOME directory.
	expectedFile := filepath.Join(tempHome, ".snap-commit", repoKey(repoRoot), "snapshots.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected snapshots file at %s: %v", expectedFile, err)
	}
}
