package core

import (
	"fmt"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/logger"
	"github.com/spf13/cobra"
)

var (
	clearFlag    bool
	clearAllFlag bool
)

type LogListResponse struct {
	Repositories []string `json:"repositories"`
}

type LogShowResponse struct {
	Repository string                 `json:"repository"`
	Entries    []logger.ActivityEntry `json:"entries"`
}

type LogClearResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func NewLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log [repo_path]",
		Short: "Manage activity logs (JSON output)",
		Long:  `List, show, or clear structured activity logs for repositories.`,
		Run: func(cmd *cobra.Command, args []string) {
			l := logger.NewActivityLogger()

			if clearAllFlag {
				err := l.ClearAll()
				if err != nil {
					PrintErrorJSON("log", err)
					return
				}
				PrintJSON("log", LogClearResponse{Success: true, Message: "All logs cleared"})
				return
			}

			repoPath := ""
			if len(args) == 1 {
				repoPath = args[0]
				absPath, err := filepath.Abs(repoPath)
				if err != nil {
					PrintErrorJSON("log", err)
					return
				}
				repoPath = absPath
			}

			if clearFlag {
				if repoPath == "" {
					PrintErrorJSON("log", fmt.Errorf("repo_path is required for --clear"))
					return
				}
				err := l.Clear(repoPath)
				if err != nil {
					PrintErrorJSON("log", err)
					return
				}
				PrintJSON("log", LogClearResponse{Success: true, Message: fmt.Sprintf("Logs for %s cleared", repoPath)})
				return
			}

			if repoPath == "" {
				// List repos with logs
				repos := l.ListRepos()
				PrintJSON("log", LogListResponse{Repositories: repos})
			} else {
				// Show logs for repo
				entries, err := l.GetLogs(repoPath)
				if err != nil {
					PrintErrorJSON("log", err)
					return
				}
				PrintJSON("log", LogShowResponse{Repository: repoPath, Entries: entries})
			}
		},
	}

	cmd.Flags().BoolVar(&clearFlag, "clear", false, "Clear logs for the specified repository")
	cmd.Flags().BoolVar(&clearAllFlag, "clear-all", false, "Clear all activity logs")

	return cmd
}
