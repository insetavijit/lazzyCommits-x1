package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// RepoInfo holds basic information about a discovered repository.
type RepoInfo struct {
	Path           string   `json:"path"`
	Branch         string   `json:"branch"`
	Commits        int      `json:"commits"`
	IsDirty        bool     `json:"isDirty"`
	Ignored        bool     `json:"ignored"`
	UntrackedCount int      `json:"untrackedCount"`
	ModifiedCount  int      `json:"modifiedCount"`
	StagedCount    int      `json:"stagedCount"`
	UntrackedFiles []string `json:"untrackedFiles,omitempty"`
	ModifiedFiles  []string `json:"modifiedFiles,omitempty"`
	StagedFiles    []string `json:"stagedFiles,omitempty"`
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

// briefConcurrent performs GetRepoBrief for multiple paths in parallel.
func briefConcurrent(paths []string) []RepoBrief {
	if len(paths) == 0 {
		return nil
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > len(paths) {
		numWorkers = len(paths)
	}

	pathChan := make(chan string, len(paths))
	resultChan := make(chan RepoBrief, len(paths))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range pathChan {
				resultChan <- GetRepoBrief(path)
			}
		}()
	}

	// Feed paths
	for _, path := range paths {
		pathChan <- path
	}
	close(pathChan)

	// Wait for workers in a separate goroutine to close resultChan
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var results []RepoBrief
	for res := range resultChan {
		results = append(results, res)
	}

	return results
}

// Scan searches for Git repositories downwards from the given root directory.
func Scan(root string) ([]RepoBrief, error) {
	var paths []string
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
			paths = append(paths, filepath.Dir(path))
			return filepath.SkipDir
		}
		if name == "node_modules" || name == "vendor" || name == ".idea" || name == ".vscode" {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return briefConcurrent(paths), nil
}

// ScanAll searches for Git repositories including parent directories of the root.
func ScanAll(root string) ([]RepoBrief, error) {
	var paths []string
	uniquePaths := make(map[string]bool)

	// Check parents upwards
	current := root
	for {
		gitDir := filepath.Join(current, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			if !uniquePaths[current] {
				paths = append(paths, current)
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
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			return nil
		}

		name := info.Name()
		if name == ".git" {
			repoPath := filepath.Dir(path)
			if !uniquePaths[repoPath] {
				paths = append(paths, repoPath)
				uniquePaths[repoPath] = true
			}
			return filepath.SkipDir
		}
		if name == "node_modules" || name == "vendor" || name == ".idea" || name == ".vscode" {
			return filepath.SkipDir
		}
		return nil
	})

	if err != nil {
		// Log error but continue with whatever paths we found
	}

	return briefConcurrent(paths), nil
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

	// Check for .lazyignore
	if _, err := os.Stat(filepath.Join(info.Path, ".lazyignore")); err == nil {
		info.Ignored = true
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
			for path, s := range status {
				if s.Staging != git.Unmodified {
					info.StagedCount++
					info.StagedFiles = append(info.StagedFiles, path)
					info.IsDirty = true
				}
				if s.Worktree == git.Untracked {
					info.UntrackedCount++
					info.UntrackedFiles = append(info.UntrackedFiles, path)
					info.IsDirty = true
				} else if s.Worktree != git.Unmodified {
					info.ModifiedCount++
					info.ModifiedFiles = append(info.ModifiedFiles, path)
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
