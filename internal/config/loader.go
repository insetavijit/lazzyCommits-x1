package config

import (
	"fmt"

	"github.com/spf13/viper"
)

func Load() (*Config, error) {
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set defaults if not provided
	if cfg.Global.PushDelaySeconds == 0 {
		cfg.Global.PushDelaySeconds = 5
	}
	if cfg.Global.IdleBeforeCommitMinutes == 0 {
		cfg.Global.IdleBeforeCommitMinutes = 5
	}
	if cfg.Global.MaxFileSizeMB == 0 {
		cfg.Global.MaxFileSizeMB = 50
	}
	if cfg.Global.LogRetentionDays == 0 {
		cfg.Global.LogRetentionDays = 30
	}

	return &cfg, nil
}
