package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [dir]",
	Short: "Scan for Git repositories downwards from a directory",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		root := "."
		if len(args) == 1 {
			root = args[0]
		}

		absRoot, err := filepath.Abs(root)
		if err != nil {
			fmt.Printf("Error resolving path: %v\n", err)
			return
		}

		fmt.Printf("Scanning for Git repositories in: %s\n", absRoot)
		repos, err := scanner.Scan(absRoot)
		if err != nil {
			fmt.Printf("Error scanning: %v\n", err)
			return
		}

		if len(repos) == 0 {
			fmt.Println("No Git repositories found.")
			return
		}

		fmt.Printf("Found %d repositories:\n\n", len(repos))
		fmt.Printf("%-50s | %-15s | %-7s | %s\n", "PATH", "BRANCH", "COMMITS", "STATUS")
		fmt.Println(filepath.Join("--------------------------------------------------", "--------------------------------------------------"))

		for _, repo := range repos {
			status := "CLEAN"
			if repo.IsDirty {
				status = fmt.Sprintf("DIRTY (S:%d M:%d U:%d)", repo.Staged, repo.Modified, repo.Untracked)
			}
			fmt.Printf("%-50s | %-15s | %-7d | %s\n", truncateRepoPath(repo.Path, 50), repo.Branch, repo.Commits, status)
		}
	},
}

func truncateRepoPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
