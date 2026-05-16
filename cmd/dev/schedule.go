package dev

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/spf13/cobra"
)

var (
	listFlag      bool
	commitFlag    bool
	pushFlag      bool
	terminateFlag string
	runFlag       string
	timeFlag      string
)

type ScheduleResponse struct {
	Repo    string             `json:"repo"`
	Success bool               `json:"success"`
	ID      string             `json:"id,omitempty"`
	Type    string             `json:"type"`
	Delay   string             `json:"delay"`
	Brief   scanner.RepoBrief  `json:"brief,omitempty"`
	Message string             `json:"message,omitempty"`
}

type TerminateResponse struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func NewScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule [repo_path]",
		Short: "Schedule or terminate a Git action (JSON output)",
		Long: `Schedule a commit, push, or any shell command. 
Or terminate a scheduled task by ID.
Examples:
  lazycommit schedule . --commit -t 5s
  lazycommit schedule --terminate 12345
  lazycommit schedule --list`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := ipc.NewClient()
			if err != nil {
				core.PrintErrorJSON("schedule", err)
				return
			}

			if terminateFlag != "" {
				resp, err := client.TerminateTask(terminateFlag)
				if err != nil {
					core.PrintErrorJSON("schedule", err)
					return
				}
				if !resp.Success {
					core.PrintErrorJSON("schedule", fmt.Errorf(resp.Error))
					return
				}
				core.PrintJSON("schedule", TerminateResponse{
					ID:      terminateFlag,
					Success: true,
					Message: "Task terminated successfully",
				})
				return
			}

			if listFlag {
				resp, err := client.GetScheduledTasks()
				if err != nil {
					core.PrintErrorJSON("schedule", err)
					return
				}
				core.PrintJSON("schedule", resp)
				return
			}

			repoPath := "."
			if len(args) == 1 {
				repoPath = args[0]
			}
			absPath, err := filepath.Abs(repoPath)
			if err != nil {
				core.PrintErrorJSON("schedule", err)
				return
			}

			var taskType string
			var taskArgs []string

			if commitFlag {
				taskType = "commit"
				msg := time.Now().Format("20060102-150405") + "-autoCommit"
				taskArgs = []string{msg}
			} else if pushFlag {
				taskType = "push"
			} else if runFlag != "" {
				taskType = "run"
				taskArgs = []string{runFlag}
			} else {
				core.PrintErrorJSON("schedule", fmt.Errorf("either --commit, --push, or --run must be specified"))
				return
			}

			resp, err := client.ScheduleTask(absPath, taskType, timeFlag, taskArgs)
			if err != nil {
				core.PrintErrorJSON("schedule", err)
				return
			}

			if !resp.Success {
				core.PrintErrorJSON("schedule", fmt.Errorf(resp.Error))
				return
			}

			// Get maximum info for the caller
			brief := scanner.GetRepoBrief(absPath)

			core.PrintJSON("schedule", ScheduleResponse{
				Repo:    absPath,
				Success: true,
				ID:      resp.ID,
				Type:    taskType,
				Delay:   timeFlag,
				Brief:   brief,
				Message: fmt.Sprintf("Successfully scheduled %s", taskType),
			})
		},
	}

	cmd.Flags().BoolVarP(&listFlag, "list", "l", false, "List all scheduled tasks")
	cmd.Flags().BoolVarP(&commitFlag, "commit", "c", false, "Schedule a commit")
	cmd.Flags().BoolVarP(&pushFlag, "push", "p", false, "Schedule a push")
	cmd.Flags().StringVarP(&terminateFlag, "terminate", "X", "", "Terminate a scheduled task by ID")
	cmd.Flags().StringVarP(&runFlag, "run", "r", "", "Schedule a custom shell command")
	cmd.Flags().StringVarP(&timeFlag, "time", "t", "5s", "Delay before execution (e.g. 5s, 2m, 1h)")

	return cmd
}
