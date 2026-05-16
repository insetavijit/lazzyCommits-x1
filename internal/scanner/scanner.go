package scanner

import (
	"os"
	"path/filepath"
)

// Scan searches for Git repositories downwards from the given root directory.
func Scan(root string) ([]string, error) {
	var repos []string
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
			repos = append(repos, filepath.Dir(path))
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
func ScanAll(root string) ([]string, error) {
	var repos []string
	
	// Check parents upwards
	current := root
	for {
		gitDir := filepath.Join(current, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			repos = append(repos, current)
			// Usually if a parent is a repo, it might be a monorepo or we just found the root.
			// The user wants "exact location", so we keep searching.
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

	// Avoid duplicates (if root itself was found both ways)
	uniqueRepos := make(map[string]bool)
	for _, r := range repos {
		uniqueRepos[r] = true
	}
	for _, r := range downstream {
		uniqueRepos[r] = true
	}

	var result []string
	for r := range uniqueRepos {
		result = append(result, r)
	}

	return result, nil
}
