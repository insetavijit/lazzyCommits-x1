package core

import (
	"fmt"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/git"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push [repo_path]",
		Short: "Perform a one-off push with safety checks",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repoPath := "."
			if len(args) == 1 {
				repoPath = args[0]
			}

			absPath, err := filepath.Abs(repoPath)
			if err != nil {
				fmt.Printf("Invalid path: %v\n", err)
				return
			}

			logger, _ := zap.NewProduction()
			defer logger.Sync()

			engine := git.NewEngine(logger)
			guard := git.NewSafetyGuard(logger)

			fmt.Printf("Performing atomic push for: %s\n", absPath)

			res := guard.Check(absPath, []string{"main", "master"}, 50)
			if !res.Passed {
				fmt.Printf("Safety Check Failed: %s\n", res.Reason)
				return
			}

			err = engine.Push(absPath)
			if err != nil {
				fmt.Printf("Push Failed: %v\n", err)
				return
			}

			fmt.Println("Push successful!")
		},
	}
}
