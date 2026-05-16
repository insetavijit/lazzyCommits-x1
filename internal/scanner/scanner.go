package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// RepoInfo holds basic information about a discovered repository.
type RepoInfo struct {
	Path      string
	Branch    string
	Commits   int
	IsDirty   bool
	Untracked int
	Modified  int
	Staged    int
}

// RemoteInfo holds remote name and URL.
type RemoteInfo struct {
	Name string   `json:"name"`
	URLs []string `json:"urls"`
}

// LastCommitInfo holds metadata about the latest commit.
type LastCommitInfo struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// RepoBrief holds comprehensive information about a repository.
type RepoBrief struct {
	RepoInfo
	Remotes    []RemoteInfo   `json:"remotes"`
	LastCommit LastCommitInfo `json:"lastCommit"`
	Unpushed   int            `json:"unpushed"`
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
	
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return info
	}

	// Update path to actual git root
	wt, err := repo.Worktree()
	if err == nil {
		info.Path = wt.Filesystem.Root()
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
	wt, err = repo.Worktree()
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

// GetRepoBrief extracts detailed Git metadata from a repository path.
func GetRepoBrief(path string) RepoBrief {
	repo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err == nil {
		if wt, err := repo.Worktree(); err == nil {
			path = wt.Filesystem.Root()
		}
	}

	base := GetRepoInfo(path)
	brief := RepoBrief{RepoInfo: base}

	if repo == nil {
		return brief
	}

	// Get Remotes
	remotes, err := repo.Remotes()
	if err == nil {
		for _, r := range remotes {
			brief.Remotes = append(brief.Remotes, RemoteInfo{
				Name: r.Config().Name,
				URLs: r.Config().URLs,
			})
		}
	}

	// Get Last Commit
	head, err := repo.Head()
	if err == nil {
		commit, err := repo.CommitObject(head.Hash())
		if err == nil {
			brief.LastCommit = LastCommitInfo{
				Hash:    commit.Hash.String(),
				Author:  fmt.Sprintf("%s <%s>", commit.Author.Name, commit.Author.Email),
				Date:    commit.Author.When.Format(time.RFC3339),
				Message: commit.Message,
			}
		}

		// Calculate Unpushed (compare HEAD with origin/branch)
		remoteRefName := plumbing.ReferenceName("refs/remotes/origin/" + head.Name().Short())
		remoteRef, err := repo.Reference(remoteRefName, true)
		if err == nil {
			// Count commits between remote and local
			cIter, err := repo.Log(&git.LogOptions{From: head.Hash()})
			if err == nil {
				count := 0
				_ = cIter.ForEach(func(c *object.Commit) error {
					if c.Hash == remoteRef.Hash() {
						return fmt.Errorf("stop")
					}
					count++
					return nil
				})
				brief.Unpushed = count
			}
		}
	}

	return brief
}
