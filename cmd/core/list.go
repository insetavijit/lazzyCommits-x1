package core

import (
	"fmt"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [repo_path]",
		Short: "Display detailed Git status summary for a repository",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				fmt.Printf("Error resolving path: %v\n", err)
				return
			}

			repo := scanner.GetRepoInfo(absPath)
			
			fmt.Printf("\n%-50s | %-15s | %-7s | %s\n", "PATH", "BRANCH", "COMMITS", "STATUS")
			fmt.Println(filepath.Join("--------------------------------------------------", "--------------------------------------------------"))

			status := "CLEAN"
			if repo.IsDirty {
				status = fmt.Sprintf("DIRTY (S:%d M:%d U:%d)", repo.Staged, repo.Modified, repo.Untracked)
			}
			fmt.Printf("%-50s | %-15s | %-7d | %s\n", TruncateRepoPath(repo.Path, 50), repo.Branch, repo.Commits, status)
		},
	}
}
