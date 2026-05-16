package dev_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/dev"
	"github.com/stretchr/testify/assert"
)

func TestNewStatusCmd(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-status-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := dev.NewStatusCmd()
	
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
	output := buf.String()

	assert.Contains(t, output, "lazyCommit daemon is not running")
}

func TestNewStatusCmd_StalePID(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-status-stale-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create a stale PID file
	pidDir := filepath.Join(tmpDir, ".lazycommit")
	os.MkdirAll(pidDir, 0755)
	os.WriteFile(filepath.Join(pidDir, "daemon.pid"), []byte("999999"), 0644)

	cmd := dev.NewStatusCmd()
	
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
	output := buf.String()

	assert.Contains(t, output, "lazyCommit daemon is not running (stale PID file)")
}

func TestNewStatusCmd_Running(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-status-running-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create a PID file with current PID
	pidDir := filepath.Join(tmpDir, ".lazycommit")
	os.MkdirAll(pidDir, 0755)
	os.WriteFile(filepath.Join(pidDir, "daemon.pid"), []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	cmd := dev.NewStatusCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// This might fail to connect to IPC, but it will cover the "running" branch
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "lazyCommit daemon is running")
}
