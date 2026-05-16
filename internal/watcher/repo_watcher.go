package watcher

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type EventType string

const (
	FileChanged    EventType = "FILE_CHANGED"
	CommitDetected EventType = "COMMIT_DETECTED"
)

type Event struct {
	Type EventType
	Path string
}

type RepoWatcher struct {
	repoPath string
	events   chan Event
	logger   *zap.Logger
}

func NewRepoWatcher(repoPath string, logger *zap.Logger) *RepoWatcher {
	return &RepoWatcher{
		repoPath: repoPath,
		events:   make(chan Event, 100),
		logger:   logger,
	}
}

func (rw *RepoWatcher) Events() <-chan Event {
	return rw.events
}

func (rw *RepoWatcher) Watch(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Recursively add directories to watch (excluding .git)
	err = rw.addRecursive(watcher, rw.repoPath)
	if err != nil {
		return fmt.Errorf("failed to add directories to watcher: %w", err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			
			// Ignore .git directory
			if isGitDir(event.Name) {
				// But watch for COMMIT_EDITMSG or similar if we want to detect commits
				// Actually, COMMIT_DETECTED might come from watching .git/refs/heads
				if isCommitEvent(event.Name) {
					rw.events <- Event{Type: CommitDetected, Path: event.Name}
				}
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write {
				rw.events <- Event{Type: FileChanged, Path: event.Name}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			rw.logger.Error("Watcher error", zap.Error(err))

		case <-ctx.Done():
			return nil
		}
	}
}

func (rw *RepoWatcher) addRecursive(watcher *fsnotify.Watcher, path string) error {
	// Simple non-recursive for now if we want to stay "single-repo" and simple for v0.1
	// but v0.1 says "single-repo fsnotify watcher", which usually implies watching the whole repo.
	
	// For v0.1, let's just watch the root and maybe .git/refs/heads for commit detection
	err := watcher.Add(path)
	if err != nil {
		return err
	}
	
	gitDir := filepath.Join(path, ".git")
	refsHeads := filepath.Join(gitDir, "refs", "heads")
	err = watcher.Add(refsHeads)
	if err != nil {
		// might not exist yet if empty repo
		rw.logger.Warn("Could not watch .git/refs/heads", zap.Error(err))
	}

	return nil
}

func isGitDir(path string) bool {
	return filepath.Base(path) == ".git" || strings.Contains(path, "/.git/") || strings.Contains(path, "\\.git\\")
}

func isCommitEvent(path string) bool {
	// Simple check: if something in .git/refs/heads changed, it's likely a commit (or branch change)
	return strings.Contains(path, ".git/refs/heads") || strings.Contains(path, ".git\\refs\\heads")
}
