package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"go.uber.org/zap"
)

type SafetyGuard struct {
	logger *zap.Logger
}

func NewSafetyGuard(logger *zap.Logger) *SafetyGuard {
	return &SafetyGuard{
		logger: logger,
	}
}

// SafetyResult contains the result of safety checks
type SafetyResult struct {
	Passed bool
	Reason string
}

func (sg *SafetyGuard) Check(repoPath string, protectedBranches []string, maxFileSizeMB int) SafetyResult {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return SafetyResult{Passed: false, Reason: fmt.Sprintf("failed to open repo: %v", err)}
	}

	// 1-3. Check for merge/rebase in progress
	gitDir := filepath.Join(repoPath, ".git")
	for _, file := range []string{"MERGE_HEAD", "REBASE_MERGE", "REBASE_APPLY"} {
		if _, err := os.Stat(filepath.Join(gitDir, file)); err == nil {
			return SafetyResult{Passed: false, Reason: fmt.Sprintf("%s exists, operation in progress", file)}
		}
	}

	// 4. HEAD is detached
	head, err := repo.Head()
	if err != nil {
		return SafetyResult{Passed: false, Reason: fmt.Sprintf("failed to get HEAD: %v", err)}
	}
	if head.Name() == "HEAD" {
		return SafetyResult{Passed: false, Reason: "HEAD is detached"}
	}

	// 5. Staged files exist in index
	worktree, err := repo.Worktree()
	if err != nil {
		return SafetyResult{Passed: false, Reason: fmt.Sprintf("failed to get worktree: %v", err)}
	}
	status, err := worktree.Status()
	if err != nil {
		return SafetyResult{Passed: false, Reason: fmt.Sprintf("failed to get status: %v", err)}
	}
	for _, s := range status {
		if s.Staging != git.Unmodified && s.Staging != git.Untracked {
			return SafetyResult{Passed: false, Reason: "staged files exist in index"}
		}
	}

	// 6. Branch is in protectedBranches
	branchName := head.Name().Short()
	for _, protected := range protectedBranches {
		if branchName == protected {
			return SafetyResult{Passed: false, Reason: fmt.Sprintf("branch %s is protected", branchName)}
		}
	}

	// 7. Changeset includes untracked file (only modified/deleted files should be auto-committed)
	// 8. Any changed file exceeds 50 MB
	for path, s := range status {
		if s.Worktree == git.Untracked {
			return SafetyResult{Passed: false, Reason: fmt.Sprintf("changeset includes untracked file: %s", path)}
		}
		if s.Worktree != git.Unmodified {
			info, err := os.Stat(filepath.Join(repoPath, path))
			if err == nil {
				if info.Size() > int64(maxFileSizeMB)*1024*1024 {
					return SafetyResult{Passed: false, Reason: fmt.Sprintf("file %s exceeds %d MB", path, maxFileSizeMB)}
				}
			}
		}
	}

	// 9. Local branch has diverged from remote (ahead AND behind)
	diverged, err := sg.isDiverged(repo, head)
	if err != nil {
		sg.logger.Warn("Failed to check if diverged", zap.Error(err))
	} else if diverged {
		return SafetyResult{Passed: false, Reason: "local branch has diverged from remote (ahead and behind)"}
	}

	return SafetyResult{Passed: true}
}

func (sg *SafetyGuard) isDiverged(repo *git.Repository, head *plumbing.Reference) (bool, error) {
	remoteRefName := plumbing.ReferenceName("refs/remotes/origin/" + head.Name().Short())
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return false, nil // No remote branch, not diverged
		}
		return false, err
	}

	// Check if we are ahead
	ahead, err := sg.isAncestor(repo, remoteRef.Hash(), head.Hash())
	if err != nil {
		return false, err
	}

	// Check if we are behind
	behind, err := sg.isAncestor(repo, head.Hash(), remoteRef.Hash())
	if err != nil {
		return false, err
	}

	// Diverged means we have commits that remote doesn't, AND remote has commits that we don't.
	// That is, head is not an ancestor of remoteRef AND remoteRef is not an ancestor of head.
	// Wait, "ahead and behind" means both are true if we consider they have a common ancestor but neither is an ancestor of the other.
	
	// If head is ancestor of remoteRef, we are behind.
	// If remoteRef is ancestor of head, we are ahead.
	// If neither, we diverged.
	
	return !ahead && !behind, nil
}

func (sg *SafetyGuard) isAncestor(repo *git.Repository, ancestor, descendant plumbing.Hash) (bool, error) {
	if ancestor == descendant {
		return true, nil
	}

	cDescendant, err := repo.CommitObject(descendant)
	if err != nil {
		return false, err
	}

	found := false
	iter, err := repo.Log(&git.LogOptions{From: cDescendant.Hash})
	if err != nil {
		return false, err
	}
	defer iter.Close()

	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == ancestor {
			found = true
			return errors.New("stop")
		}
		return nil
	})

	if err != nil && err.Error() != "stop" {
		return false, err
	}

	return found, nil
}
