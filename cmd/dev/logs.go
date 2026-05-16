package dev

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/lazycommit/lazycommit/internal/logger"
	"github.com/spf13/cobra"
)

func NewLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [repo_path]",
		Short: "View activity logs for a repository",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repoPath := ""
			if len(args) == 1 {
				repoPath = args[0]
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					fmt.Println("Error getting current directory:", err)
					return
				}
				repoPath = cwd
			}

			absPath, err := filepath.Abs(repoPath)
			if err != nil {
				fmt.Println("Invalid path:", err)
				return
			}

			home, _ := os.UserHomeDir()
			sanitized := core.SanitizeRepoPath(absPath)
			logFile := filepath.Join(home, ".lazycommit", "logs", sanitized+".log")

			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				fmt.Printf("No logs found for repository: %s\n", absPath)
				return
			}

			file, err := os.Open(logFile)
			if err != nil {
				fmt.Println("Error opening log file:", err)
				return
			}
			defer file.Close()

			fmt.Printf("Activity logs for %s:\n", absPath)
			fmt.Printf("%-25s | %-12s | %-8s | %s\n", "TIMESTAMP", "ACTION", "OUTCOME", "DETAILS")
			fmt.Println(strings.Repeat("-", 80))

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				var entry logger.ActivityEntry
				if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
					continue
				}

				ts := entry.Timestamp.Local().Format(time.RFC3339)
				action := string(entry.Action)
				outcome := string(entry.Outcome)
				details := fmt.Sprintf("%dms", entry.Duration)
				if entry.Error != "" {
					details += " | " + entry.Error
				}

				fmt.Printf("%-25s | %-12s | %-8s | %s\n", ts, action, outcome, details)
			}

			if err := scanner.Err(); err != nil {
				fmt.Println("Error reading log file:", err)
			}
		},
	}
}
