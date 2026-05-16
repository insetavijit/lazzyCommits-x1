package dev

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/spf13/cobra"
)

func NewStatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stat",
		Short: "Show active and upcoming scheduled tasks",
		Long:  `Queries the running daemon for the current schedule table, including pending commits and pushes.`,
		Run: func(cmd *cobra.Command, args []string) {
			client, err := ipc.NewClient()
			if err != nil {
				fmt.Println("Could not connect to daemon IPC:", err)
				return
			}

			resp, err := client.GetScheduledTasks()
			if err != nil {
				fmt.Println("Error fetching schedule from daemon:", err)
				return
			}

			if len(resp.Tasks) == 0 {
				fmt.Println("No upcoming tasks scheduled.")
				return
			}

			// Sort by RunAt
			sort.Slice(resp.Tasks, func(i, j int) bool {
				return resp.Tasks[i].RunAt.Before(resp.Tasks[j].RunAt)
			})

			fmt.Println("Upcoming Scheduled Tasks:")
			fmt.Printf("%-10s | %-12s | %-30s | %s\n", "ID", "TYPE", "REPOSITORY", "RUN AT")
			fmt.Println(strings.Repeat("-", 80))

			now := time.Now()
			for _, task := range resp.Tasks {
				repo := task.Repo
				if len(repo) > 30 {
					repo = "..." + repo[len(repo)-27:]
				}

				dueIn := task.RunAt.Sub(now).Round(time.Second)
				dueStr := ""
				if dueIn <= 0 {
					dueStr = "NOW"
				} else {
					dueStr = fmt.Sprintf("in %s", dueIn)
				}

				fmt.Printf("%-10s | %-12s | %-30s | %s (%s)\n", 
					task.ID, task.Type, repo, task.RunAt.Local().Format("15:04:05"), dueStr)
			}
		},
	}
}
