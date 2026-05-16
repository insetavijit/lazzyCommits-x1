package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

type RetryItem struct {
	RepoPath  string    `json:"repo_path"`
	Failures  int       `json:"failures"`
	NextRetry time.Time `json:"next_retry"`
}

type RetryQueue struct {
	filePath string
	items    []*RetryItem
	mu       sync.Mutex
	logger   *zap.Logger
}

func NewRetryQueue(logger *zap.Logger) (*RetryQueue, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".lazycommit", "retry-queue.json")
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	q := &RetryQueue{
		filePath: path,
		logger:   logger,
	}

	if err := q.load(); err != nil {
		return nil, err
	}

	return q, nil
}

func (q *RetryQueue) load() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	data, err := os.ReadFile(q.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			q.items = []*RetryItem{}
			return nil
		}
		return err
	}

	if len(data) == 0 {
		q.items = []*RetryItem{}
		return nil
	}

	return json.Unmarshal(data, &q.items)
}

func (q *RetryQueue) save() error {
	data, err := json.MarshalIndent(q.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(q.filePath, data, 0644)
}

func (q *RetryQueue) Enqueue(repoPath string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if already in queue
	for _, item := range q.items {
		if item.RepoPath == repoPath {
			return // Already queued
		}
	}

	q.items = append(q.items, &RetryItem{
		RepoPath:  repoPath,
		Failures:  0,
		NextRetry: time.Now().Add(10 * time.Second),
	})
	q.save()
}

func (q *RetryQueue) PeekReady() *RetryItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for i, item := range q.items {
		if now.After(item.NextRetry) {
			// Remove from queue for processing
			q.items = append(q.items[:i], q.items[i+1:]...)
			q.save()
			return item
		}
	}
	return nil
}

func (q *RetryQueue) HandleFailure(item *RetryItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	item.Failures++
	if item.Failures >= 5 {
		q.logger.Error("dead-letter: dropping repo from retry queue after 5 failures", zap.String("repo", item.RepoPath))
		q.save()
		return
	}

	backoffs := []time.Duration{
		10 * time.Second,
		30 * time.Second,
		2 * time.Minute,
		10 * time.Minute,
		30 * time.Minute,
	}

	item.NextRetry = time.Now().Add(backoffs[item.Failures])
	q.items = append(q.items, item)
	q.save()
}
