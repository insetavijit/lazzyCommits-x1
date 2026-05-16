package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/lazycommit/lazycommit/internal/ipc"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the lazyCommit daemon",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		pidFile := filepath.Join(home, ".lazycommit", "daemon.pid")

		isRunning := false
		var pid int
		if _, err := os.Stat(pidFile); err == nil {
			pidData, _ := os.ReadFile(pidFile)
			pid, _ = strconv.Atoi(string(pidData))
			process, err := os.FindProcess(pid)
			if err == nil {
				err := process.Signal(syscall.Signal(0))
				if err == nil {
					isRunning = true
				}
			}
		}

		if !isRunning {
			fmt.Println("lazyCommit daemon is not running")
			return
		}

		fmt.Printf("lazyCommit daemon is running (PID: %d)\n", pid)

		client, err := ipc.NewClient()
		if err != nil {
			fmt.Printf("Failed to create IPC client: %v\n", err)
			return
		}

		resp, err := client.GetStatus()
		if err != nil {
			fmt.Printf("Failed to query daemon: %v (is it really running?)\n", err)
			return
		}

		fmt.Println("\nRepository Status:")
		if len(resp.Repos) == 0 {
			fmt.Println("  No repositories configured or active.")
		}
		for _, repo := range resp.Repos {
			fmt.Printf("  %-40s [%s]\n", repo.Path, repo.State)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
