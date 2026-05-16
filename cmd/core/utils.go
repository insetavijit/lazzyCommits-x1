package core

import (
	"os"
	"strings"
)

func TruncateRepoPath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

func SanitizeRepoPath(path string) string {
	path = strings.TrimSuffix(path, string(os.PathSeparator))
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return r.Replace(path)
}
