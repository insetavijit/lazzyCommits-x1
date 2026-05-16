package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/lazycommit/lazycommit/internal/autostart"
	"github.com/spf13/cobra"
)

var installFlag bool

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the lazyCommit daemon",
		Run: func(cmd *cobra.Command, args []string) {
			if installFlag {
				err := autostart.Install()
				if err != nil {
					fmt.Printf("Failed to install autostart: %v\n", err)
					return
				}
				fmt.Println("lazyCommit autostart installed successfully.")
				return
			}

			home, _ := os.UserHomeDir()
			pidFile := filepath.Join(home, ".lazycommit", "daemon.pid")

			if _, err := os.Stat(pidFile); err == nil {
				pidData, _ := os.ReadFile(pidFile)
				pid, _ := strconv.Atoi(string(pidData))
				process, err := os.FindProcess(pid)
				if err == nil {
					err := process.Signal(syscall.Signal(0))
					if err == nil {
						fmt.Println("lazyCommit daemon is already running (PID:", pid, ")")
						return
					}
				}
			}

			executable, _ := os.Executable()
			daemonCmd := exec.Command(executable, "daemon")
			
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

	cmd.Flags().BoolVarP(&installFlag, "install", "i", false, "Install autostart script for current user")
	return cmd
}
