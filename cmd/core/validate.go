package core

import (
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/scanner"
	"github.com/lazycommit/lazycommit/internal/ssh"
	"github.com/spf13/cobra"
)

var (
	sshFlag bool
	pathFlag string
	allFlag  bool
)

type ValidateResponse struct {
	SSH    *ssh.VerifyResult `json:"ssh,omitempty"`
	Repo   *scanner.RepoInfo `json:"repo,omitempty"`
	Status string           `json:"status"`
}

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate system and project configuration (JSON output)",
		Long:  `Validates SSH connectivity to GitHub and repository health (including .lazyignore detection).`,
		Run: func(cmd *cobra.Command, args []string) {
			resp := ValidateResponse{Status: "ok"}

			if allFlag || sshFlag {
				res, _ := ssh.VerifyConnection()
				resp.SSH = res
				if !res.Success {
					resp.Status = "error"
				}
			}

			if allFlag || pathFlag != "" {
				root := "."
				if pathFlag != "" {
					root = pathFlag
				}
				absRoot, err := filepath.Abs(root)
				if err == nil {
					info := scanner.GetRepoInfo(absRoot)
					resp.Repo = &info
				}
			}

			PrintJSON("validate", resp)
		},
	}

	cmd.Flags().BoolVar(&sshFlag, "ssh", false, "Validate GitHub SSH connectivity")
	cmd.Flags().StringVar(&pathFlag, "path", "", "Validate repository at path (checks .lazyignore)")
	cmd.Flags().BoolVar(&allFlag, "all", false, "Run all validations")

	return cmd
}
