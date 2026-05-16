package core

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/lazycommit/lazycommit/internal/git"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type CommitResponse struct {
	Repo    string `json:"repo"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func NewCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit [repo_path] [message]",
		Short: "Perform a one-off auto-commit (JSON output)",
		Long: `Perform an auto-commit for the specified repository.
Path defaults to '.' if empty or omitted.
If message is 'random', a timestamped message like '20260516-093000-autoCommit' is used.`,
		Args: cobra.MaximumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			repoPath := "."
			if len(args) >= 1 && args[0] != "" {
				repoPath = args[0]
			}

			message := ""
			if len(args) == 2 {
				if args[1] == "random" {
					message = time.Now().Format("20060102-150405") + "-autoCommit"
				} else {
					message = args[1]
				}
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

			err = engine.StageAndCommitWithMsg(absPath, message)
			if err != nil {
				PrintErrorJSON(err)
				return
			}

			PrintJSON(CommitResponse{
				Repo:    absPath,
				Success: true,
				Message: "Auto-commit successful",
			})
		},
	}
}
