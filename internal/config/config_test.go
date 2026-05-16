package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create a temporary directory for config
	tmpDir, err := os.MkdirTemp("", "lazycommit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.toml")
	configContent := `
[global]
push_delay_seconds = 10
idle_before_commit_minutes = 2
notifications = true

[[repos]]
path = "/path/to/repo1"
enabled = true
lazy_push = true
protected_branches = ["main"]

[[repos]]
path = "/path/to/repo2"
enabled = false
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set viper to use this file
	viper.Reset()
	viper.SetConfigFile(configPath)
	err = viper.ReadInConfig()
	require.NoError(t, err)

	cfg, err := Load()
	require.NoError(t, err)

	// Assert global config
	assert.Equal(t, 10, cfg.Global.PushDelaySeconds)
	assert.Equal(t, 2, cfg.Global.IdleBeforeCommitMinutes)
	assert.True(t, cfg.Global.Notifications)

	// Assert repos
	require.Len(t, cfg.Repos, 2)
	assert.Equal(t, "/path/to/repo1", cfg.Repos[0].Path)
	assert.True(t, cfg.Repos[0].Enabled)
	assert.True(t, cfg.Repos[0].LazyPush)
	assert.Equal(t, []string{"main"}, cfg.Repos[0].ProtectedBranches)

	assert.Equal(t, "/path/to/repo2", cfg.Repos[1].Path)
	assert.False(t, cfg.Repos[1].Enabled)
}

func TestLoadDefaults(t *testing.T) {
	viper.Reset()
	// No config file, just unmarshal empty
	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, 5, cfg.Global.PushDelaySeconds)
	assert.Equal(t, 5, cfg.Global.IdleBeforeCommitMinutes)
	assert.Equal(t, 50, cfg.Global.MaxFileSizeMB)
	assert.Equal(t, 30, cfg.Global.LogRetentionDays)
}
