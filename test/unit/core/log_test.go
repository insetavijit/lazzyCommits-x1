package core_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/stretchr/testify/assert"
)

func TestNewLogCmd_List(t *testing.T) {
	// Mock HOME to avoid polluting real home
	tmpDir, _ := os.MkdirTemp("", "lazycommit-log-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := core.NewLogCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.Bytes()

	var envelope core.ResponseEnvelope
	err = json.Unmarshal(output, &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "log", envelope.Command)
	
	// Data should be LogListResponse
	dataMap := envelope.Data.(map[string]interface{})
	assert.Contains(t, dataMap, "repositories")
}

func TestNewLogCmd_ClearAll(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-log-clearall-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := core.NewLogCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--clear-all"})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.Bytes()

	var envelope core.ResponseEnvelope
	err = json.Unmarshal(output, &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "log", envelope.Command)
	
	dataMap := envelope.Data.(map[string]interface{})
	assert.True(t, dataMap["success"].(bool))
}

func TestNewLogCmd_ClearOne(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-log-clearone-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create a dummy log file
	logDir := filepath.Join(tmpDir, ".lazycommit", "logs")
	os.MkdirAll(logDir, 0755)
	repoPath := "/tmp/test-repo"
	sanitized := core.SanitizeRepoPath(repoPath)
	os.WriteFile(filepath.Join(logDir, sanitized+".log"), []byte("{}"), 0644)

	cmd := core.NewLogCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--clear", repoPath})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.Bytes()

	var envelope core.ResponseEnvelope
	err = json.Unmarshal(output, &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "log", envelope.Command)
	
	dataMap := envelope.Data.(map[string]interface{})
	assert.True(t, dataMap["success"].(bool))
}

func TestNewLogCmd_Show(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-log-show-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create a dummy log file
	logDir := filepath.Join(tmpDir, ".lazycommit", "logs")
	os.MkdirAll(logDir, 0755)
	repoPath := "/tmp/test-repo"
	sanitized := core.SanitizeRepoPath(repoPath)
	os.WriteFile(filepath.Join(logDir, sanitized+".log"), []byte(`{"id":"1","ts":"2026-05-16T11:00:00Z","action":"COMMITTED","outcome":"SUCCESS","durationMs":100}`+"\n"), 0644)

	cmd := core.NewLogCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{repoPath})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.Bytes()

	var envelope core.ResponseEnvelope
	err = json.Unmarshal(output, &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "log", envelope.Command)
	
	dataMap := envelope.Data.(map[string]interface{})
	assert.Equal(t, repoPath, dataMap["repository"])
	entries := dataMap["entries"].([]interface{})
	assert.Len(t, entries, 1)
}
