package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSafetyGuard_Check(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "safety-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	repo, err := git.PlainInit(repoDir, false)
	require.NoError(t, err)

	sg := NewSafetyGuard(zap.NewNop())

	// Test 1: MERGE_HEAD exists
	err = os.WriteFile(filepath.Join(repoDir, ".git", "MERGE_HEAD"), []byte(""), 0644)
	require.NoError(t, err)
	res := sg.Check(repoDir, []string{}, 50)
	assert.False(t, res.Passed)
	assert.Contains(t, res.Reason, "MERGE_HEAD exists")
	os.Remove(filepath.Join(repoDir, ".git", "MERGE_HEAD"))

	// Need a commit to have a branch
	wt, err := repo.Worktree()
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("data"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("file.txt")
	require.NoError(t, err)
	_, err = wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "e", When: time.Now()},
	})
	require.NoError(t, err)

	// Test 6: Protected branch
	res = sg.Check(repoDir, []string{"master", "main"}, 50)
	assert.False(t, res.Passed)
	assert.Contains(t, res.Reason, "is protected")

	// Test 5: Staged files
	err = os.WriteFile(filepath.Join(repoDir, "staged.txt"), []byte("data"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("staged.txt")
	require.NoError(t, err)
	res = sg.Check(repoDir, []string{}, 50)
	assert.False(t, res.Passed)
	assert.Contains(t, res.Reason, "staged files exist")
	
	// Unstage for next test
	// go-git doesn't have a simple "reset", but we can just commit it or leave it.
	// Actually, let's just commit it to clear the index.
	_, err = wt.Commit("clear staged", &git.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "e", When: time.Now()},
	})
	require.NoError(t, err)

	// Test 7: Untracked file
	err = os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("data"), 0644)
	require.NoError(t, err)
	res = sg.Check(repoDir, []string{}, 50)
	assert.False(t, res.Passed)
	assert.Contains(t, res.Reason, "includes untracked file")
	os.Remove(filepath.Join(repoDir, "untracked.txt"))

	// Test 8: Large file
	// First commit a small version so it's tracked
	err = os.WriteFile(filepath.Join(repoDir, "large.txt"), []byte("small"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("large.txt")
	require.NoError(t, err)
	_, err = wt.Commit("add large.txt", &git.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "e", When: time.Now()},
	})
	require.NoError(t, err)

	// Now make it large and modified
	err = os.WriteFile(filepath.Join(repoDir, "large.txt"), make([]byte, 2*1024*1024), 0644)
	require.NoError(t, err)
	
	res = sg.Check(repoDir, []string{}, 1) // 1MB limit
	assert.False(t, res.Passed)
	assert.Contains(t, res.Reason, "exceeds 1 MB")
}
