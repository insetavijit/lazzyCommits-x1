package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/spf13/cobra"
)

var randomMessages = []string{
	"auto: making the world better, one byte at a time",
	"auto: I forgot to commit this earlier",
	"auto: magic happens here",
	"auto: just keep swimming",
	"auto: trust me, I'm a daemon",
	"auto: fix it, ship it, forget it",
	"auto: oops, did I do that?",
	"auto: automated progress",
}

var listFlag bool

var scheduleCmd = &cobra.Command{
	Use:   "schedule [duration] [message]",
	Short: "Schedule a manual auto-commit via the daemon",
	Long: `Schedule a commit for the current repository after a specific delay.
The 'message' argument can be 'random' to pick a funny automated message.
Use --list to see all currently scheduled commits.`,
	Args: cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := ipc.NewClient()
		if err != nil {
			fmt.Println("Error connecting to daemon:", err)
			return
		}

		if listFlag {
			resp, err := client.GetStatus()
			if err != nil {
				fmt.Printf("Failed to get scheduled list: %v\n", err)
				return
			}

			fmt.Println("Currently Scheduled Auto-Commits:")
			fmt.Printf("%-50s | %-20s | %s\n", "REPO", "SCHEDULED AT", "MESSAGE")
			fmt.Println(strings.Repeat("-", 100))

			found := false
			for _, repo := range resp.Repos {
				if !repo.ScheduledAt.IsZero() {
					fmt.Printf("%-50s | %-20s | %s\n", truncateRepoPath(repo.Path, 50), repo.ScheduledAt.Local().Format("15:04:05"), repo.ScheduledMsg)
					found = true
				}
			}

			if !found {
				fmt.Println("No commits currently scheduled.")
			}
			return
		}

		if len(args) < 1 {
			fmt.Println("Error: duration argument is required when not using --list")
			return
		}

		delay := args[0]
		message := "auto: manual scheduled commit"
		if len(args) > 1 {
			if args[1] == "random" {
				message = randomMessages[rand.Intn(len(randomMessages))]
			} else {
				message = args[1]
			}
		}

		cwd, _ := os.Getwd()
		absPath, _ := filepath.Abs(cwd)

		resp, err := client.ScheduleCommit(absPath, delay, message)
		if err != nil {
			fmt.Printf("Failed to schedule commit: %v\n", err)
			return
		}

		if resp.Success {
			fmt.Printf("Successfully scheduled commit for %s in %s with message: %s\n", absPath, delay, message)
		} else {
			fmt.Printf("Error from daemon: %s\n", resp.Error)
		}
	},
}

func init() {
	scheduleCmd.Flags().BoolVarP(&listFlag, "list", "l", false, "List all scheduled commits")
	rootCmd.AddCommand(scheduleCmd)
}
