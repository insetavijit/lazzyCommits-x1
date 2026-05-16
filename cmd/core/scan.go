package core

import (
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

func NewScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan [dir]",
		Short: "Scan for Git repositories (JSON output)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}

			absRoot, err := filepath.Abs(root)
			if err != nil {
				PrintErrorJSON(err)
				return
			}

			repos, err := scanner.Scan(absRoot)
			if err != nil {
				PrintErrorJSON(err)
				return
			}

			PrintJSON(repos)
		},
	}
}
