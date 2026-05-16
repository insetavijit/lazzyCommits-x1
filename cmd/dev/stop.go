package dev

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

func NewStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the lazyCommit daemon",
		Run: func(cmd *cobra.Command, args []string) {
			home, _ := os.UserHomeDir()
			pidFile := filepath.Join(home, ".lazycommit", "daemon.pid")

			if _, err := os.Stat(pidFile); err == nil {
				pidData, _ := os.ReadFile(pidFile)
				pid, _ := strconv.Atoi(string(pidData))
				process, err := os.FindProcess(pid)
				if err == nil {
					err = process.Signal(syscall.SIGTERM)
					if err == nil {
						fmt.Println("lazyCommit daemon stopped (PID:", pid, ")")
						return
					}
				}
			}

			fmt.Println("lazyCommit daemon is not running")
		},
	}
}
