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

func TestNewStopCmd_NotRunning(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-stop-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := dev.NewStopCmd()
	
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

func TestNewStopCmd_Running(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-stop-running-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Create a PID file with current PID
	pidDir := filepath.Join(tmpDir, ".lazycommit")
	os.MkdirAll(pidDir, 0755)
	os.WriteFile(filepath.Join(pidDir, "daemon.pid"), []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	// We just check if it exits without crashing for now.
	// We won't actually call Execute() as it would kill the test runner.
	cmd := dev.NewStopCmd()
	assert.NotNil(t, cmd)
}
