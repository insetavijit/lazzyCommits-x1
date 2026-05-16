package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the lazyCommit daemon",
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
		if err != nil {
			fmt.Printf("Failed to find process: %v\n", err)
			return
		}

		err = process.Signal(syscall.SIGTERM)
		if err != nil {
			fmt.Printf("Failed to stop daemon: %v\n", err)
			return
		}

		fmt.Println("lazyCommit daemon stopped (PID:", pid, ")")
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
