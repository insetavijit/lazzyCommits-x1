package core

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/lazycommit/lazycommit/internal/watcher"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type WatchEvent struct {
	Repo  string `json:"repo"`
	Type  string `json:"type"`
	Path  string `json:"path"`
}

func NewWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch [path1] [path2] ...",
		Short: "Watch multiple paths for file changes (JSON output)",
		Long: `Starts a concurrent foreground watcher for one or more paths.
Each path is monitored in its own goroutine for maximum performance.
Outputs real-time file change events as structured JSON.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			logger := zap.NewNop()
			
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigChan
				cancel()
			}()

			var wg sync.WaitGroup
			eventChan := make(chan WatchEvent)

			// Start a goroutine for each provided path
			for _, path := range args {
				absPath, err := filepath.Abs(path)
				if err != nil {
					continue
				}

				wg.Add(1)
				go func(p string) {
					defer wg.Done()
					rw := watcher.NewRepoWatcher(p, logger)
					
					// Start the internal watcher loop
					go func() {
						if err := rw.Watch(ctx); err != nil {
							// Logger is Nop, but we could handle errors here
						}
					}()

					// Stream events to our central collector
					for {
						select {
						case ev := <-rw.Events():
							eventChan <- WatchEvent{
								Repo: p,
								Type: string(ev.Type),
								Path: ev.Path,
							}
						case <-ctx.Done():
							return
						}
					}
				}(absPath)
			}

			// Printer goroutine
			go func() {
				for ev := range eventChan {
					PrintJSON("watch", ev)
				}
			}()

			wg.Wait()
			close(eventChan)
		},
	}
}
