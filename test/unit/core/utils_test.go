package core_test

import (
	"testing"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/stretchr/testify/assert"
)

func TestTruncateRepoPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		maxLen   int
		expected string
	}{
		{
			name:     "path shorter than maxLen",
			path:     "/home/user/repo",
			maxLen:   20,
			expected: "/home/user/repo",
		},
		{
			name:     "path equals maxLen",
			path:     "/home/user/repo",
			maxLen:   15,
			expected: "/home/user/repo",
		},
		{
			name:     "path longer than maxLen",
			path:     "/home/user/very/long/path/to/repo",
			maxLen:   15,
			expected: "...path/to/repo",
		},
		{
			name:     "very short maxLen",
			path:     "/home/user/repo",
			maxLen:   5,
			expected: "...po",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := core.TruncateRepoPath(tt.path, tt.maxLen)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestSanitizeRepoPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "standard path",
			path:     "/home/user/repo",
			expected: "_home_user_repo",
		},
		{
			name:     "path with trailing slash",
			path:     "/home/user/repo/",
			expected: "_home_user_repo",
		},
		{
			name:     "windows path",
			path:     "C:\\Users\\User\\repo",
			expected: "C__Users_User_repo",
		},
		{
			name:     "complex path",
			path:     "/path/with:special\\chars/",
			expected: "_path_with_special_chars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := core.SanitizeRepoPath(tt.path)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
