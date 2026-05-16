package core

import (
	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/spf13/cobra"
)

func NewStatCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stat-json",
		Short: "Show active and upcoming scheduled tasks (JSON output)",
		Run: func(cmd *cobra.Command, args []string) {
			client, err := ipc.NewClient()
			if err != nil {
				PrintErrorJSON("stat", err)
				return
			}

			resp, err := client.GetScheduledTasks()
			if err != nil {
				PrintErrorJSON("stat", err)
				return
			}

			PrintJSON("stat", resp)
		},
	}
}
