package core

import (
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [repo_path]",
		Short: "Display detailed Git status summary (JSON output)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				PrintErrorJSON(err)
				return
			}

			repo := scanner.GetRepoInfo(absPath)
			PrintJSON(repo)
		},
	}
}
