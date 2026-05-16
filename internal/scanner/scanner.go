package scanner

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// RepoInfo holds detailed information about a discovered repository.
type RepoInfo struct {
	Path        string
	Branch      string
	Commits     int
	IsDirty     bool
	Untracked   int
	Modified    int
	Staged      int
}

// Scan searches for Git repositories downwards from the given root directory.
func Scan(root string) ([]RepoInfo, error) {
	var repos []RepoInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			return nil
		}

		// Skip common directories that won't contain repos or are too large
		name := info.Name()
		if name == ".git" {
			repoPath := filepath.Dir(path)
			repoInfo := GetRepoInfo(repoPath)
			repos = append(repos, repoInfo)
			return filepath.SkipDir
		}
		if name == "node_modules" || name == "vendor" || name == ".idea" || name == ".vscode" {
			return filepath.SkipDir
		}

		return nil
	})
	return repos, err
}

// ScanAll searches for Git repositories including parent directories of the root.
func ScanAll(root string) ([]RepoInfo, error) {
	var repos []RepoInfo
	
	uniquePaths := make(map[string]bool)

	// Check parents upwards
	current := root
	for {
		gitDir := filepath.Join(current, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			if !uniquePaths[current] {
				repos = append(repos, GetRepoInfo(current))
				uniquePaths[current] = true
			}
		}
		
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	// Scan downwards from root
	downstream, err := Scan(root)
	if err != nil {
		return repos, err
	}

	for _, info := range downstream {
		if !uniquePaths[info.Path] {
			repos = append(repos, info)
			uniquePaths[info.Path] = true
		}
	}

	return repos, nil
}

// GetRepoInfo extracts Git metadata from a repository path.
func GetRepoInfo(path string) RepoInfo {
	info := RepoInfo{Path: path, Branch: "unknown"}
	
	repo, err := git.PlainOpen(path)
	if err != nil {
		return info
	}

	// Get Branch
	head, err := repo.Head()
	if err == nil {
		info.Branch = head.Name().Short()
	}

	// Count total commits
	cIter, err := repo.Log(&git.LogOptions{All: true})
	if err == nil {
		count := 0
		_ = cIter.ForEach(func(c *object.Commit) error {
			count++
			return nil
		})
		info.Commits = count
	}

	// Get status for dirty check and counts
	wt, err := repo.Worktree()
	if err == nil {
		status, err := wt.Status()
		if err == nil {
			for _, s := range status {
				if s.Staging != git.Unmodified {
					info.Staged++
					info.IsDirty = true
				}
				if s.Worktree == git.Untracked {
					info.Untracked++
					info.IsDirty = true
				} else if s.Worktree != git.Unmodified {
					info.Modified++
					info.IsDirty = true
				}
			}
		}
	}

	return info
}
