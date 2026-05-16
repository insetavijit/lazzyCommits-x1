package daemon

import (
	"context"

	"github.com/fsnotify/fsnotify"
	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func (d *Daemon) watchConfig(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		d.logger.Error("Failed to create config watcher", zap.Error(err))
		return
	}
	defer watcher.Close()

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		d.logger.Warn("No config file used, config watcher disabled")
		return
	}

	err = watcher.Add(configFile)
	if err != nil {
		d.logger.Error("Failed to add config file to watcher", zap.String("file", configFile), zap.Error(err))
		return
	}

	d.logger.Info("Watching config file for changes", zap.String("file", configFile))

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				d.logger.Info("Config file changed, reloading", zap.String("file", event.Name))
				newCfg, err := config.Load()
				if err != nil {
					d.logger.Error("Failed to reload config", zap.Error(err))
					continue
				}
				d.Reconcile(newCfg)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			d.logger.Error("Config watcher error", zap.Error(err))
		case <-ctx.Done():
			return
		}
	}
}
