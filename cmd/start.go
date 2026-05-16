package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/lazycommit/lazycommit/internal/autostart"
)

var installFlag bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the lazyCommit daemon",
	Run: func(cmd *cobra.Command, args []string) {
		if installFlag {
			err := autostart.Install()
			if err != nil {
				fmt.Printf("Failed to install autostart: %v\n", err)
				return
			}
			fmt.Println("lazyCommit daemon installed and started via system autostart")
			return
		}

		home, _ := os.UserHomeDir()
		pidFile := filepath.Join(home, ".lazycommit", "daemon.pid")

		if _, err := os.Stat(pidFile); err == nil {
			pidData, _ := os.ReadFile(pidFile)
			pid, _ := strconv.Atoi(string(pidData))
			process, err := os.FindProcess(pid)
			if err == nil {
				// Check if process is actually running
				err := process.Signal(syscall.Signal(0))
				if err == nil {
					fmt.Println("lazyCommit daemon is already running (PID:", pid, ")")
					return
				}
			}
		}

		// Start daemon in background
		executable, _ := os.Executable()
		daemonCmd := exec.Command(executable, "daemon")
		
		// Redirect output to log files or /dev/null
		logDir := filepath.Join(home, ".lazycommit", "logs")
		os.MkdirAll(logDir, 0755)
		stdout, _ := os.OpenFile(filepath.Join(logDir, "daemon.stdout"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		stderr, _ := os.OpenFile(filepath.Join(logDir, "daemon.stderr"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		
		daemonCmd.Stdout = stdout
		daemonCmd.Stderr = stderr
		daemonCmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
		
		err := daemonCmd.Start()
		if err != nil {
			fmt.Printf("Failed to start daemon: %v\n", err)
			return
		}

		fmt.Println("lazyCommit daemon started (PID:", daemonCmd.Process.Pid, ")")
	},
}

func init() {
	startCmd.Flags().BoolVarP(&installFlag, "install", "i", false, "Install lazyCommit daemon to start on login")
	rootCmd.AddCommand(startCmd)
}
