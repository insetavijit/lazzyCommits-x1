package git

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/lazycommit/lazycommit/internal/composer"
	"go.uber.org/zap"
)

type GitEngine interface {
	Push(repoPath string) error
	HasUnpushedCommits(repoPath string) (bool, error)
	StageAndCommit(repoPath string) error
	StageAndCommitWithMsg(repoPath string, msg string) error
}

type Engine struct {
	logger *zap.Logger
}

func NewEngine(logger *zap.Logger) *Engine {
	return &Engine{
		logger: logger,
	}
}

func (e *Engine) StageAndCommit(repoPath string) error {
	return e.StageAndCommitWithMsg(repoPath, "")
}

func (e *Engine) StageAndCommitWithMsg(repoPath string, msg string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	// Stage tracked files that are modified or deleted, and untracked files
	stagedStatus := make(git.Status)
	hasChanges := false
	for path, s := range status {
		if s.Worktree == git.Modified || s.Worktree == git.Deleted || s.Worktree == git.Untracked {
			_, err := worktree.Add(path)
			if err != nil {
				return fmt.Errorf("failed to stage file %s: %w", path, err)
			}
			stagedStatus[path] = s
			hasChanges = true
		}
	}

	// Submodule support: check for changed submodules
	submodules, err := worktree.Submodules()
	if err == nil {
		for _, sub := range submodules {
			status, err := sub.Status()
			if err == nil && !status.IsClean() {
				// Stage the submodule pointer change
				_, err := worktree.Add(sub.Config().Path)
				if err == nil {
					e.logger.Info("Staged submodule change", zap.String("submodule", sub.Config().Path))
					hasChanges = true
				}
			}
		}
	}

	if len(stagedStatus) == 0 && !hasChanges {
		e.logger.Info("No changes to commit", zap.String("path", repoPath))
		return nil
	}

	finalMsg := msg
	if finalMsg == "" {
		finalMsg = composer.Compose(stagedStatus)
	}

	_, err = worktree.Commit(finalMsg, &git.CommitOptions{})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	e.logger.Info("Successfully committed changes", zap.String("path", repoPath), zap.String("msg", finalMsg))
	return nil
}

func (e *Engine) Push(repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	auth, err := e.getAuth()
	if err != nil {
		e.logger.Warn("Failed to get auth, trying without", zap.Error(err))
	}

	err = repo.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			e.logger.Info("Repo already up to date", zap.String("path", repoPath))
			return nil
		}
		return fmt.Errorf("failed to push: %w", err)
	}

	e.logger.Info("Successfully pushed", zap.String("path", repoPath))
	return nil
}

func (e *Engine) getAuth() (ssh.AuthMethod, error) {
	// Try SSH agent first
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		// go-git uses SSH agent by default if no auth is provided for SSH URLs, 
		// but we can be explicit if we want.
		// However, let's try to load the default key if agent is not there.
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshDir := filepath.Join(home, ".ssh")
	keys := []string{"id_ed25519", "id_rsa"}
	for _, keyName := range keys {
		keyPath := filepath.Join(sshDir, keyName)
		if _, err := os.Stat(keyPath); err == nil {
			publicKeys, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
			if err == nil {
				return publicKeys, nil
			}
			e.logger.Warn("Failed to load SSH key", zap.String("path", keyPath), zap.Error(err))
		}
	}

	return nil, fmt.Errorf("no SSH keys found")
}

func (e *Engine) HasUnpushedCommits(repoPath string) (bool, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false, fmt.Errorf("failed to open repo: %w", err)
	}

	// This is a bit simplified. To check if unpushed, we usually compare local branch with remote.
	// For v0.1, we can just try to push or do a more sophisticated check.
	
	// A common way with go-git:
	head, err := repo.Head()
	if err != nil {
		return false, err
	}

	_, err = repo.Remote("origin")
	if err != nil {
		return false, err
	}

	// Fetch remote references to compare
	// But we don't want to pull/fetch as per "What lazyCommit is not"
	// "Push only. Never pulls, never fetches."
	
	// However, to know if we are ahead, we need to know what the remote has.
	// If we don't fetch, we can only rely on what we last knew about the remote (remotes/origin/main).
	
	remoteRefName := plumbing.ReferenceName("refs/remotes/origin/" + head.Name().Short())
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		if err.Error() == "reference not found" {
			// Remote branch doesn't exist? Assume we need to push.
			return true, nil
		}
		return false, err
	}

	return head.Hash() != remoteRef.Hash(), nil
}
