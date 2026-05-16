package dev

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lazycommit/lazycommit/internal/logger"
	"github.com/spf13/cobra"
)

func NewHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Show a global history of all daemon activities",
		Long:  `Displays a chronological list of all commits and pushes performed by the daemon across all repositories.`,
		Run: func(cmd *cobra.Command, args []string) {
			l := logger.NewActivityLogger()
			allEntries, err := l.GetAllLogs()
			if err != nil {
				fmt.Println("Error reading history:", err)
				return
			}

			if len(allEntries) == 0 {
				fmt.Println("No activity history found.")
				return
			}

			// Sort by timestamp descending
			sort.Slice(allEntries, func(i, j int) bool {
				return allEntries[i].Timestamp.After(allEntries[j].Timestamp)
			})

			// Limit to 20 for readability
			limit := 20
			if len(allEntries) < limit {
				limit = len(allEntries)
			}

			fmt.Printf("Recent Daemon Activity (Last %d entries):\n", limit)
			fmt.Printf("%-20s | %-25s | %-12s | %-8s | %s\n", "TIMESTAMP", "REPOSITORY", "ACTION", "OUTCOME", "DETAILS")
			fmt.Println(strings.Repeat("-", 100))

			for i := 0; i < limit; i++ {
				entry := allEntries[i]
				ts := entry.Timestamp.Local().Format("2006-01-02 15:04:05")
				repo := entry.Repo
				if len(repo) > 25 {
					parts := strings.Split(repo, "/")
					if len(parts) > 2 {
						repo = ".../" + parts[len(parts)-2] + "/" + parts[len(parts)-1]
					}
					if len(repo) > 25 {
						repo = "..." + repo[len(repo)-22:]
					}
				}
				
				action := string(entry.Action)
				outcome := string(entry.Outcome)
				details := fmt.Sprintf("%dms", entry.Duration)
				if entry.Error != "" {
					details += " | " + entry.Error
				}

				fmt.Printf("%-20s | %-25s | %-12s | %-8s | %s\n", ts, repo, action, outcome, details)
			}
		},
	}
}
