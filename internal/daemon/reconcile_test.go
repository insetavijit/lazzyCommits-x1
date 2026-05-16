package daemon

import (
	"context"
	"testing"

	"github.com/lazycommit/lazycommit/internal/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestReconcile(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/repo1", Enabled: true},
		},
	}
	d := New(cfg, logger)
	d.ctx = context.Background()

	// Initial setup
	_, cancel := context.WithCancel(d.ctx)
	d.registry.Add(cfg.Repos[0], cancel)

	assert.Len(t, d.registry.All(), 1)

	// 1. Add a new repo
	newCfg := &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/repo1", Enabled: true},
			{Path: "/repo2", Enabled: true},
		},
	}
	d.Reconcile(newCfg)
	assert.Len(t, d.registry.All(), 2)
	_, ok := d.registry.Get("/repo2")
	assert.True(t, ok)

	// 2. Disable a repo
	newCfg = &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/repo1", Enabled: true},
			{Path: "/repo2", Enabled: false},
		},
	}
	d.Reconcile(newCfg)
	assert.Len(t, d.registry.All(), 1)
	_, ok = d.registry.Get("/repo2")
	assert.False(t, ok)

	// 3. Remove a repo
	newCfg = &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/repo1", Enabled: true},
		},
	}
	d.Reconcile(newCfg)
	assert.Len(t, d.registry.All(), 1)

	// 4. Update repo config
	newCfg = &config.Config{
		Repos: []config.RepoConfig{
			{Path: "/repo1", Enabled: true, LazyPush: true},
		},
	}
	d.Reconcile(newCfg)
	assert.Len(t, d.registry.All(), 1)
	repo, _ := d.registry.Get("/repo1")
	assert.True(t, repo.Config.LazyPush)
}
