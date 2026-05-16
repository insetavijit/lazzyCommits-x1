package logger
import (
	"encoding/json"
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
	Timestamp time.Time `json:"ts"`
	Action    Action    `json:"action"`
	Outcome   Outcome   `json:"outcome"`
	Duration  int64     `json:"durationMs"`
	Error     string    `json:"error,omitempty"`
}

type ActivityLogger struct{}

func NewActivityLogger() *ActivityLogger {
	return &ActivityLogger{}
}

func (l *ActivityLogger) Log(repoPath string, action Action, outcome Outcome, duration time.Duration, err error) {
	entry := ActivityEntry{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Outcome:   outcome,
		Duration:  duration.Milliseconds(),
	}
	if err != nil {
		entry.Error = err.Error()
	}

	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".lazycommit", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return // Best effort
	}

	sanitized := sanitizeRepoPath(repoPath)
	logFile := filepath.Join(logDir, sanitized+".log")

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

func sanitizeRepoPath(path string) string {
	// Remove trailing slash
	path = strings.TrimSuffix(path, string(os.PathSeparator))
	// Replace slashes and other non-friendly characters with underscores
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return r.Replace(path)
}
