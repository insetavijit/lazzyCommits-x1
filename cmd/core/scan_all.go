package core

import (
	"fmt"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

func NewScanAllCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan-all [dir]",
		Short: "Scan for Git repositories including parent directories",
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

			fmt.Printf("Scanning (all) for Git repositories starting at: %s\n", absRoot)
			repos, err := scanner.ScanAll(absRoot)
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
				fmt.Printf("%-50s | %-15s | %-7d | %s\n", TruncateRepoPath(repo.Path, 50), repo.Branch, repo.Commits, status)
			}
		},
	}
}
