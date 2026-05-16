package core

import (
	"sort"

	"github.com/lazycommit/lazycommit/internal/logger"
	"github.com/spf13/cobra"
)

func NewHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history-json",
		Short: "Show global activity history (JSON output)",
		Run: func(cmd *cobra.Command, args []string) {
			l := logger.NewActivityLogger()
			allEntries, err := l.GetAllLogs()
			if err != nil {
				PrintErrorJSON("history", err)
				return
			}

			// Sort by timestamp descending
			sort.Slice(allEntries, func(i, j int) bool {
				return allEntries[i].Timestamp.After(allEntries[j].Timestamp)
			})

			PrintJSON("history", allEntries)
		},
	}
}
