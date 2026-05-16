package composer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
)

// Compose generates a commit message based on the git status.
// All messages start with "auto: ".
// 1 file: "auto: modified src/auth.go"
// 2-3 files: "auto: modified auth.go, added auth_test.go"
// 4+ files: "auto: 4 files changed"
func Compose(status git.Status) string {
	if len(status) == 0 {
		return "auto: no changes"
	}

	if len(status) >= 4 {
		return fmt.Sprintf("auto: %d files changed", len(status))
	}

	var parts []string
	for path, s := range status {
		verb := "modified"
		if s.Worktree == git.Deleted || s.Staging == git.Deleted {
			verb = "deleted"
		} else if s.Worktree == git.Untracked || s.Worktree == git.Added || s.Staging == git.Added {
			verb = "added"
		} else if s.Worktree == git.Modified || s.Staging == git.Modified {
			verb = "modified"
		}
		parts = append(parts, fmt.Sprintf("%s %s", verb, path))
	}

	// Sort parts to ensure deterministic output
	sort.Strings(parts)

	return "auto: " + strings.Join(parts, ", ")
}
