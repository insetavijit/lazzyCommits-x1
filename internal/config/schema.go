package config

type GlobalConfig struct {
	PushDelaySeconds        int  `mapstructure:"push_delay_seconds"`
	IdleBeforeCommitMinutes int  `mapstructure:"idle_before_commit_minutes"`
	Notifications           bool `mapstructure:"notifications"`
	LogRetentionDays        int  `mapstructure:"log_retention_days"`
	MaxFileSizeMB           int  `mapstructure:"max_file_size_mb"`
}

type RepoOverrides struct {
	PushDelaySeconds        int `mapstructure:"push_delay_seconds"`
	IdleBeforeCommitMinutes int `mapstructure:"idle_before_commit_minutes"`
}

type RepoConfig struct {
	Path             string         `mapstructure:"path"`
	Enabled          bool           `mapstructure:"enabled"`
	LazyPush         bool           `mapstructure:"lazy_push"`
	LazyCommit       bool           `mapstructure:"lazy_commit"`
	ProtectedBranches []string       `mapstructure:"protected_branches"`
	IgnorePatterns   []string       `mapstructure:"ignore_patterns"`
	Overrides        RepoOverrides  `mapstructure:"overrides"`
}

type Config struct {
	Global GlobalConfig `mapstructure:"global"`
	Repos  []RepoConfig `mapstructure:"repos"`
}
