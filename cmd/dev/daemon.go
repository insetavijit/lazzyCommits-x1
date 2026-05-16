package dev

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/lazycommit/lazycommit/internal/daemon"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var daemonDebugFlag bool

func NewDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "daemon",
		Short:  "Run the lazyCommit daemon",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}

			var logger *zap.Logger
			if daemonDebugFlag {
				loggerConfig := zap.NewDevelopmentConfig()
				logger, _ = loggerConfig.Build()
			} else {
				logger, _ = zap.NewProduction()
			}
			defer logger.Sync()

			d := daemon.New(cfg, logger)

			home, _ := os.UserHomeDir()
			pidDir := filepath.Join(home, ".lazycommit")
			os.MkdirAll(pidDir, 0755)
			pidFile := filepath.Join(pidDir, "daemon.pid")
			
			err = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
			if err != nil {
				logger.Fatal("Failed to write PID file", zap.Error(err))
			}
			defer os.Remove(pidFile)

			if err := d.Start(context.Background()); err != nil {
				logger.Fatal("Daemon failed", zap.Error(err))
			}
		},
	}

	cmd.Flags().BoolVarP(&daemonDebugFlag, "debug", "d", false, "Enable debug logging")
	return cmd
}
