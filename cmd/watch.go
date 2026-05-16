package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/lazycommit/lazycommit/internal/watcher"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var watchCmd = &cobra.Command{
	Use:   "watch [repo_path]",
	Short: "Run a foreground watcher to debug file events",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repoPath := "."
		if len(args) == 1 {
			repoPath = args[0]
		}

		absPath, err := filepath.Abs(repoPath)
		if err != nil {
			fmt.Printf("Invalid path: %v\n", err)
			return
		}

		logger, _ := zap.NewProduction()
		defer logger.Sync()

		rw := watcher.NewRepoWatcher(absPath, logger)
		
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println("\nStopping watcher...")
			cancel()
		}()

		fmt.Printf("Starting foreground atomic watcher for: %s\n", absPath)
		fmt.Println("Press Ctrl+C to stop.")

		go func() {
			for event := range rw.Events() {
				fmt.Printf("[%s] Event: %s Path: %s\n", 
					filepath.Base(absPath), event.Type, event.Path)
			}
		}()

		if err := rw.Watch(ctx); err != nil {
			fmt.Printf("Watcher Error: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
