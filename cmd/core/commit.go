package core

import (
	"path/filepath"
	"time"

	"github.com/lazycommit/lazycommit/internal/git"
	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type CommitResponse struct {
	Repo    string             `json:"repo"`
	Success bool               `json:"success"`
	Message string             `json:"message,omitempty"`
	Brief   scanner.RepoBrief  `json:"brief,omitempty"`
}

func NewCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit [repo_path] [message]",
		Short: "Perform a one-off auto-commit with full metadata response (JSON)",
		Long: `Perform an auto-commit for the specified repository.
Path defaults to '.' if empty or omitted.
If message is 'random', a timestamped message is used.
This command executes directly (bypassing background safety guards) to provide atomic service.`,
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
			
			// Execute commit blindly
			err = engine.StageAndCommitWithMsg(absPath, message)
			// We don't exit immediately on error because we still want to return the brief if possible
			success := true
			errMsg := "Auto-commit successful"
			if err != nil {
				success = false
				errMsg = err.Error()
			}

			// Get maximum info for the caller
			brief := scanner.GetRepoBrief(absPath)

			PrintJSON(CommitResponse{
				Repo:    absPath,
				Success: success,
				Message: errMsg,
				Brief:   brief,
			})
		},
	}
}
