package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHasUnpushedCommits(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	remoteDir := filepath.Join(tmpDir, "remote")
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(remoteDir, 0755)
	require.NoError(t, err)

	// Init bare remote
	_, err = git.PlainInit(remoteDir, true)
	require.NoError(t, err)

	// Init local repo
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	engine := NewEngine(zap.NewNop())

	// No commits yet
	has, err := engine.HasUnpushedCommits(repoDir)
	assert.Error(t, err) // Should error because HEAD is not set
	assert.False(t, has)

	// Add a commit
	wt, err := repo.Worktree()
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("hello"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("test.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)

	// Add remote
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	})
	require.NoError(t, err)

	// Now unpushed
	has, err = engine.HasUnpushedCommits(repoDir)
	require.NoError(t, err)
	assert.True(t, has)

	// Push
	err = repo.Push(&git.PushOptions{RemoteName: "origin"})
	require.NoError(t, err)

	// Now up to date
	has, err = engine.HasUnpushedCommits(repoDir)
	require.NoError(t, err)
	assert.False(t, has)
}
