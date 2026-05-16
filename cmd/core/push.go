package core

import (
	"fmt"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/git"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type PushResponse struct {
	Repo    string `json:"repo"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func NewPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push [repo_path]",
		Short: "Perform a one-off push with safety checks (JSON output)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repoPath := "."
			if len(args) == 1 {
				repoPath = args[0]
			}

			absPath, err := filepath.Abs(repoPath)
			if err != nil {
				PrintErrorJSON(err)
				return
			}

			logger := zap.NewNop()
			engine := git.NewEngine(logger)
			guard := git.NewSafetyGuard(logger)

			res := guard.Check(absPath, []string{"main", "master"}, 50)
			if !res.Passed {
				PrintErrorJSON(fmt.Errorf("safety check failed: %s", res.Reason))
				return
			}

			err = engine.Push(absPath)
			if err != nil {
				PrintErrorJSON(err)
				return
			}

			PrintJSON(PushResponse{
				Repo:    absPath,
				Success: true,
				Message: "Push successful",
			})
		},
	}
}
