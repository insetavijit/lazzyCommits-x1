package core

import (
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

func NewBriefCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "brief [repo_path]",
		Short: "Display a comprehensive Git repository briefing (JSON output)",
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

			brief := scanner.GetRepoBrief(absPath)
			PrintJSON(brief)
		},
	}
}
