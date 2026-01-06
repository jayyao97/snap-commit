# snap-commit

> Lightweight Git snapshots without polluting your repository history.

`snap-commit` creates _ghost commits_ that capture the exact state of your working tree
and staging area. Use it as a safety net before risky refactors, while pairing with AI
tools, or whenever you want versioned checkpoints without touching your actual branches.

## Highlights

- **No Git pollution** – snapshots live as orphaned commits via a temporary index.
- **Complete capture** – staged, unstaged, new, and deleted files are all recorded.
- **Safe restores** – restores both worktree and index and scrubs new untracked files.
- **Global history** – snapshots are tracked in `~/.snap-commit/snapshots.json`.
- **Interactive UX** – browse, filter, and delete snapshots directly from the TUI.

## Installation

Requires Go 1.25 or newer.

```bash
go install github.com/jayyao97/snap-commit@latest
```

Or build from source:

```bash
git clone https://github.com/jayyao97/snap-commit.git
cd snap-commit
go build -o snap-commit .
```

## Quick Start

```bash
# Create a snapshot with an optional message
snap-commit store -m "before refactoring auth module"

# Browse snapshots for the current repository (↑/↓ to move, d to delete)
snap-commit list

# Restore the latest snapshot and remove it afterwards (undo)
snap-commit restore -d
```

## Commands

### `store`

```bash
snap-commit store [-m MESSAGE]
```

Creates a new ghost commit capturing the current working tree and index. When no
message is provided `"snap-commit snapshot"` is used.

### `list`

```bash
snap-commit list [-a]
```

Launches an interactive picker for the current repository. Use `-a` to browse snapshots
from every repository tracked in the storage directory.

### `restore`

```bash
snap-commit restore [INDEX|ID]
snap-commit restore --id ID
snap-commit restore --commit SHA
snap-commit restore -d [INDEX|ID]
# Restore by snapshot ID (commit prefix + timestamp)
snap-commit restore 4d22f9c4a1bkg01z0w0m2
```

Restores files and index state from the chosen snapshot. `-d` removes the snapshot after a
successful restore (handy for “undo” workflows).

## Interactive List Shortcuts

| Key | Action |
| --- | --- |
| `↑/↓` | Navigate snapshots |
| `/` | Filter snapshots |
| `enter` | Select snapshot in restore mode |
| `d` | Delete the highlighted snapshot |
| `esc`/`q`/`ctrl+c` | Exit the view |

Deletion persists immediately by updating `snapshots.json` on disk.

## How It Works

Inspired by [Codex](https://github.com/openai/codex), the workflow is:

1. Create a temporary index via `GIT_INDEX_FILE`.
2. Run `git read-tree HEAD` and `git add --all` to prepare the tree.
3. Write a tree object(`git write-tree`) and an orphaned commit with `git commit-tree`.
4. Persist metadata (repository, message, timestamps, untracked paths) in local storage.
5. Restore with `git restore --worktree --staged` and remove any post-snapshot untracked files.

Ghost commits are never referenced by branches or tags, so your primary Git history stays clean
while snapshots remain accessible until garbage collected.

## Storage Layout

Snapshots are grouped by repository under `~/.snap-commit/<repo-hash>/snapshots.json` with entries
containing:

- Snapshot ID (`<commit-prefix><base36 timestamp>`)
- Commit SHA and parent SHA
- Message and timestamp
- Repository root path
- Pre-existing untracked files and directories

## Limitations

- Snapshots rely on Git’s object database; `git gc` can prune orphaned commits after ~14 days.
- Only works inside Git repositories.
- Snapshots cannot be restored across different repositories.

## Contributing

Issues and pull requests are welcome. Please run `go fmt ./...` and sanity-check `go test ./...`
before submitting changes.

## License

MIT License — see [LICENSE](LICENSE).
