package dev_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/lazycommit/lazycommit/cmd/dev"
	"github.com/stretchr/testify/assert"
)

func TestNewLogsCmd_NoLogs(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-logs-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := dev.NewLogsCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"."})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "No logs found for repository")
}

func TestNewLogsCmd_WithLogs(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-logs-with-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	absPath, _ := filepath.Abs(".")
	sanitized := core.SanitizeRepoPath(absPath)
	logDir := filepath.Join(tmpDir, ".lazycommit", "logs")
	os.MkdirAll(logDir, 0755)
	os.WriteFile(filepath.Join(logDir, sanitized+".log"), []byte(`{"id":"1","ts":"2026-05-16T11:00:00Z","action":"COMMITTED","outcome":"SUCCESS","durationMs":100}`+"\n"), 0644)

	cmd := dev.NewLogsCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"."})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Activity logs for")
	assert.Contains(t, output, "COMMITTED")
	assert.Contains(t, output, "SUCCESS")
}
