package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

var scanAllCmd = &cobra.Command{
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

		fmt.Printf("Found %d repositories:\n", len(repos))
		for _, repo := range repos {
			fmt.Println(repo)
		}
	},
}

func init() {
	rootCmd.AddCommand(scanAllCmd)
}
