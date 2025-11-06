package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jayyao97/snap-commit/cmd"
)

const version = "0.1.0"

func printUsage() {
	fmt.Printf(`snap-commit v%s - Git snapshot tool without polluting git state

Usage:
  snap-commit store [-m MESSAGE]       Create a new snapshot
  snap-commit restore [ID|INDEX]       Restore to a snapshot (latest or interactive select)
  snap-commit restore -d [ID|INDEX]    Restore and delete the snapshot
  snap-commit restore --id ID          Restore by ID without prompts
  snap-commit restore --commit SHA     Restore by commit without prompts
  snap-commit list                     List snapshots for current repository
  snap-commit list -a                  List all snapshots
  snap-commit help                     Show this help message
  snap-commit version                  Show version

Examples:
  snap-commit store                    Create snapshot with default message
  snap-commit store -m "before refactor"  Create snapshot with custom message
  snap-commit restore                  Restore to latest snapshot
  snap-commit restore 3                Restore to 3rd snapshot (by index)
  snap-commit restore -d               Restore latest and delete it (like undo)
  snap-commit list                     List all snapshots

How it works:
  1. Uses temporary Git index (GIT_INDEX_FILE) to avoid polluting git state
  2. Creates ghost commits that aren't referenced by any branch
  3. Stores metadata in ~/.snap-commit/snapshots.json
  4. Restores both working tree and index to snapshot state
`, version)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "store":
		storeCmd := flag.NewFlagSet("store", flag.ExitOnError)
		message := storeCmd.String("m", "", "Snapshot message")
		storeCmd.Parse(os.Args[2:])

		if err := cmd.StoreCommand(*message); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "restore":
		restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
		restoreID := restoreCmd.String("id", "", "Restore snapshot by ID directly")
		restoreCommit := restoreCmd.String("commit", "", "Restore snapshot by commit SHA")
		deleteSnapshot := restoreCmd.Bool("d", false, "Delete snapshot after restore")
		restoreCmd.Parse(os.Args[2:])

		if *restoreID != "" && *restoreCommit != "" {
			fmt.Fprintln(os.Stderr, "Error: specify only one of --id or --commit")
			os.Exit(1)
		}

		identifier := ""
		if restoreCmd.NArg() > 0 {
			identifier = restoreCmd.Arg(0)
		}

		if err := cmd.RestoreCommand(identifier, *restoreID, *restoreCommit, *deleteSnapshot); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "list", "ls":
		listCmd := flag.NewFlagSet("list", flag.ExitOnError)
		allRepos := listCmd.Bool("a", false, "Show snapshots from all repositories")
		listCmd.Parse(os.Args[2:])

		if err := cmd.ListCommand(*allRepos); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "help", "-h", "--help":
		printUsage()

	case "version", "-v", "--version":
		fmt.Printf("snap-commit v%s\n", version)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}
