package dev

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var forceStop bool

func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the lazyCommit daemon",
		Run: func(cmd *cobra.Command, args []string) {
			home, _ := os.UserHomeDir()
			pidFile := filepath.Join(home, ".lazycommit", "daemon.pid")
			socketFile := filepath.Join(home, ".lazycommit", "daemon.sock")

			if _, err := os.Stat(pidFile); err != nil {
				fmt.Println("lazyCommit daemon is not running")
				cleanupStaleFiles(pidFile, socketFile)
				return
			}

			pidData, _ := os.ReadFile(pidFile)
			pid, _ := strconv.Atoi(string(pidData))
			process, err := os.FindProcess(pid)
			if err != nil {
				fmt.Println("lazyCommit daemon is not running (stale PID file)")
				cleanupStaleFiles(pidFile, socketFile)
				return
			}

			// Check if process is actually running
			err = process.Signal(syscall.Signal(0))
			if err != nil {
				fmt.Println("lazyCommit daemon is not running (stale PID file)")
				cleanupStaleFiles(pidFile, socketFile)
				return
			}

			if forceStop {
				fmt.Printf("Forcefully stopping lazyCommit daemon (PID: %d)...\n", pid)
				process.Signal(syscall.SIGKILL)
			} else {
				fmt.Printf("Stopping lazyCommit daemon (PID: %d)...\n", pid)
				process.Signal(syscall.SIGTERM)
			}

			// Wait for process to exit
			for i := 0; i < 10; i++ {
				err = process.Signal(syscall.Signal(0))
				if err != nil {
					fmt.Println("lazyCommit daemon stopped successfully")
					cleanupStaleFiles(pidFile, socketFile)
					return
				}
				time.Sleep(500 * time.Millisecond)
			}

			if !forceStop {
				fmt.Println("Daemon is taking too long to stop. Use --force to kill it.")
			} else {
				fmt.Println("Failed to kill daemon process.")
			}
		},
	}

	cmd.Flags().BoolVarP(&forceStop, "force", "f", false, "Forcefully stop the daemon using SIGKILL")
	return cmd
}

func cleanupStaleFiles(pidFile, socketFile string) {
	if _, err := os.Stat(pidFile); err == nil {
		os.Remove(pidFile)
	}
	if _, err := os.Stat(socketFile); err == nil {
		os.Remove(socketFile)
	}
}
