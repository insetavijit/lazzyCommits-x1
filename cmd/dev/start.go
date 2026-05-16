package dev

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/lazycommit/lazycommit/internal/autostart"
	"github.com/lazycommit/lazycommit/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	installFlag bool
	debugFlag   bool
)

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the lazyCommit daemon",
		Run: func(cmd *cobra.Command, args []string) {
			// Step 1: Validate SSH before starting
			fmt.Println("Validating GitHub SSH connection...")
			verify, err := ssh.VerifyConnection()
			if err != nil || !verify.Success {
				fmt.Printf("SSH Validation Failed: %v\n", verify.Message)
				fmt.Println("Please run 'lazycommit ssh [email]' to setup your keys.")
				return
			}
			fmt.Printf("Authenticated as: %s\n", verify.User)

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
			daemonArgs := []string{"daemon"}
			if debugFlag {
				daemonArgs = append(daemonArgs, "--debug")
			}
			daemonCmd := exec.Command(executable, daemonArgs...)
			
			logDir := filepath.Join(home, ".lazycommit", "logs")
			os.MkdirAll(logDir, 0755)
			stdout, _ := os.OpenFile(filepath.Join(logDir, "daemon.stdout"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			stderr, _ := os.OpenFile(filepath.Join(logDir, "daemon.stderr"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			
			daemonCmd.Stdout = stdout
			daemonCmd.Stderr = stderr
			daemonCmd.SysProcAttr = &syscall.SysProcAttr{
				Setsid: true,
			}
			
			err = daemonCmd.Start()
			if err != nil {
				fmt.Printf("Failed to start daemon: %v\n", err)
				return
			}

			fmt.Println("lazyCommit daemon started (PID:", daemonCmd.Process.Pid, ")")
		},
	}

	cmd.Flags().BoolVarP(&installFlag, "install", "i", false, "Install autostart script for current user")
	cmd.Flags().BoolVarP(&debugFlag, "debug", "d", false, "Enable debug logging")
	return cmd
}
