package core

import (
	"github.com/lazycommit/lazycommit/internal/ssh"
	"github.com/spf13/cobra"
)

func NewSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh [email]",
		Short: "Setup SSH configuration for GitHub (JSON output)",
		Long:  `Automatically generates an Ed25519 key, adds GitHub to known_hosts, and configures ~/.ssh/config.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			email := args[0]
			res, err := ssh.Setup(email)
			if err != nil {
				PrintErrorJSON(err)
				return
			}
			PrintJSON(res)
		},
	}
}
