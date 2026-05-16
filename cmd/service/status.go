package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/spf13/cobra"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the status of the lazyCommit daemon",
		Run: func(cmd *cobra.Command, args []string) {
			home, _ := os.UserHomeDir()
			pidFile := filepath.Join(home, ".lazycommit", "daemon.pid")

			if _, err := os.Stat(pidFile); err != nil {
				fmt.Println("lazyCommit daemon is not running")
				return
			}

			pidData, _ := os.ReadFile(pidFile)
			pid, _ := strconv.Atoi(string(pidData))
			process, err := os.FindProcess(pid)
			if err == nil {
				err := process.Signal(syscall.Signal(0))
				if err == nil {
					fmt.Printf("lazyCommit daemon is running (PID: %d)\n", pid)
					
					client, err := ipc.NewClient()
					if err != nil {
						fmt.Println("Could not connect to daemon IPC:", err)
						return
					}

					resp, err := client.GetStatus()
					if err != nil {
						fmt.Println("Error fetching status from daemon:", err)
						return
					}

					fmt.Println("\nRepository Status:")
					for _, repo := range resp.Repos {
						fmt.Printf("  %s [%s]\n", repo.Path, repo.State)
					}
					return
				}
			}

			fmt.Println("lazyCommit daemon is not running (stale PID file)")
		},
	}
}
