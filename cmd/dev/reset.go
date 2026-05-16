package dev

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func NewResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Restart the lazyCommit daemon and reload configuration",
		Run: func(cmd *cobra.Command, args []string) {
			executable, _ := os.Executable()

			fmt.Println("Resetting lazyCommit daemon...")

			// 1. Stop existing daemon
			stopCmd := exec.Command(executable, "stop")
			stopCmd.Stdout = os.Stdout
			stopCmd.Stderr = os.Stderr
			stopCmd.Run()

			// 2. Start new daemon
			startCmd := exec.Command(executable, "start")
			startCmd.Stdout = os.Stdout
			startCmd.Stderr = os.Stderr
			if err := startCmd.Run(); err != nil {
				fmt.Printf("Failed to restart daemon: %v\n", err)
				return
			}

			fmt.Println("lazyCommit daemon reset successfully")
		},
	}
}
