package dev_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/dev"
	"github.com/stretchr/testify/assert"
)

func TestNewScheduleCmd_Error(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-schedule-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := dev.NewScheduleCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run without flags should fail
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.NoError(t, err) // Cobra Run doesn't return error in this case, it calls core.PrintErrorJSON

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "either --commit, --push, or --run must be specified")
}
