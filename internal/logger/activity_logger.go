package logger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Action string

const (
	ActionCommitted Action = "COMMITTED"
	ActionPushed    Action = "PUSHED"
	ActionError     Action = "ERROR"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "SUCCESS"
	OutcomeFailed  Outcome = "FAILED"
)

type ActivityEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"ts"`
	Action    Action    `json:"action"`
	Outcome   Outcome   `json:"outcome"`
	Duration  int64     `json:"durationMs"`
	Error     string    `json:"error,omitempty"`
}

type ActivityLogger struct {
	logDir string
}

func NewActivityLogger() *ActivityLogger {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".lazycommit", "logs")
	os.MkdirAll(logDir, 0755)
	return &ActivityLogger{logDir: logDir}
}

func (l *ActivityLogger) Log(repoPath string, action Action, outcome Outcome, duration time.Duration, err error) {
	entry := ActivityEntry{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Action:    action,
		Outcome:   outcome,
		Duration:  duration.Milliseconds(),
	}
	if err != nil {
		entry.Error = err.Error()
	}

	sanitized := l.SanitizeRepoPath(repoPath)
	logFile := filepath.Join(l.logDir, sanitized+".log")

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f.Write(data)
	f.Write([]byte("\n"))
}

func (l *ActivityLogger) Clear(repoPath string) error {
	sanitized := l.SanitizeRepoPath(repoPath)
	logFile := filepath.Join(l.logDir, sanitized+".log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return fmt.Errorf("no logs found for repository: %s", repoPath)
	}
	return os.Remove(logFile)
}

func (l *ActivityLogger) ClearAll() error {
	return os.RemoveAll(l.logDir)
}

func (l *ActivityLogger) GetLogs(repoPath string) ([]ActivityEntry, error) {
	sanitized := l.SanitizeRepoPath(repoPath)
	logFile := filepath.Join(l.logDir, sanitized+".log")
	
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(logFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []ActivityEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry ActivityEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func (l *ActivityLogger) ListRepos() []string {
	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		return nil
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") && entry.Name() != "daemon.log" {
			repos = append(repos, strings.TrimSuffix(entry.Name(), ".log"))
		}
	}
	return repos
}

func (l *ActivityLogger) SanitizeRepoPath(path string) string {
	path = strings.TrimSuffix(path, string(os.PathSeparator))
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return r.Replace(path)
}
