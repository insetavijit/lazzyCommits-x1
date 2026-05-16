package dev

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/spf13/cobra"
)

var (
	listFlag   bool
	commitFlag bool
	pushFlag   bool
	runFlag    string
	timeFlag   string
)

var randomMessages = []string{
	"auto: automated progress",
	"auto: making the world better, one byte at a time",
	"auto: I forgot to commit this earlier",
	"auto: magic happens here",
	"auto: just keep swimming",
	"auto: trust me, I'm a daemon",
	"auto: fix it, ship it, forget it",
	"auto: oops, did I do that?",
}

func NewScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule [repo_path]",
		Short: "Schedule a Git action or shell command via the daemon",
		Long: `Schedule a commit, push, or any shell command for a repository after a specific delay.
Examples:
  lazycommit schedule . --commit -t 5s
  lazycommit schedule . --push -t 10m
  lazycommit schedule . --run "npm run build" -t 15s`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := ipc.NewClient()
			if err != nil {
				fmt.Println("Error connecting to daemon:", err)
				return
			}

			if listFlag {
				resp, err := client.GetScheduledTasks()
				if err != nil {
					fmt.Printf("Failed to get scheduled list: %v\n", err)
					return
				}

				fmt.Println("Currently Scheduled Tasks:")
				fmt.Printf("%-20s | %-10s | %-50s | %s\n", "ID", "TYPE", "REPO", "RUN AT")
				fmt.Println(strings.Repeat("-", 110))

				if len(resp.Tasks) == 0 {
					fmt.Println("No tasks currently scheduled.")
					return
				}

				for _, t := range resp.Tasks {
					fmt.Printf("%-20s | %-10s | %-50s | %s\n", 
						t.ID, t.Type, core.TruncateRepoPath(t.Repo, 50), t.RunAt.Local().Format("15:04:05"))
				}
				return
			}

			repoPath := "."
			if len(args) == 1 {
				repoPath = args[0]
			}
			absPath, _ := filepath.Abs(repoPath)

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
				fmt.Println("Error: either --commit, --push, or --run must be specified")
				return
			}

			resp, err := client.ScheduleTask(absPath, taskType, timeFlag, taskArgs)
			if err != nil {
				fmt.Printf("Failed to schedule task: %v\n", err)
				return
			}

			if resp.Success {
				fmt.Printf("Successfully scheduled %s for %s in %s (ID: %s)\n", 
					taskType, absPath, timeFlag, resp.ID)
			} else {
				fmt.Printf("Error from daemon: %s\n", resp.Error)
			}
		},
	}

	cmd.Flags().BoolVarP(&listFlag, "list", "l", false, "List all scheduled tasks")
	cmd.Flags().BoolVarP(&commitFlag, "commit", "c", false, "Schedule a commit")
	cmd.Flags().BoolVarP(&pushFlag, "push", "p", false, "Schedule a push")
	cmd.Flags().StringVarP(&runFlag, "run", "r", "", "Schedule a custom shell command")
	cmd.Flags().StringVarP(&timeFlag, "time", "t", "5s", "Delay before execution (e.g. 5s, 2m, 1h)")

	return cmd
}
