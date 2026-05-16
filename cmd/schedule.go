package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

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

var scheduleCmd = &cobra.Command{
	Use:   "schedule [duration] [message]",
	Short: "Schedule a manual auto-commit via the daemon",
	Long: `Schedule a commit for the current repository after a specific delay.
The 'message' argument can be 'random' to pick a funny automated message.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
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

		client, err := ipc.NewClient()
		if err != nil {
			fmt.Println("Error connecting to daemon:", err)
			return
		}

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
	rootCmd.AddCommand(scheduleCmd)
}
