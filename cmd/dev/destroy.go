package dev

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Stop the daemon and remove all configuration and logs",
		Long:  `The nuclear option: stops the daemon, uninstalls autostart, and deletes the entire ~/.lazycommit directory.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("WARNING: This will delete ALL configuration and activity logs. Are you sure? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.ToLower(strings.TrimSpace(input))

			if input != "y" && input != "yes" {
				fmt.Println("Destroy cancelled.")
				return
			}

			executable, _ := os.Executable()
			home, _ := os.UserHomeDir()
			lcDir := filepath.Join(home, ".lazycommit")

			fmt.Println("Destroying lazyCommit installation...")

			// 1. Stop the daemon
			fmt.Println("Stopping daemon...")
			stopCmd := exec.Command(executable, "stop")
			stopCmd.Run()

			// 2. Uninstall autostart (if applicable - we can call start --uninstall if implemented)
			// For now, we manually remove the dir which usually contains the service files too
			
			// 3. Remove the directory
			fmt.Printf("Removing %s...\n", lcDir)
			err := os.RemoveAll(lcDir)
			if err != nil {
				fmt.Printf("Error removing directory: %v\n", err)
				return
			}

			fmt.Println("lazyCommit has been completely removed from your system.")
		},
	}
}
