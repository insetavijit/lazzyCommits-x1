package dev_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/dev"
	"github.com/stretchr/testify/assert"
)

func TestNewStartCmd(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-start-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := dev.NewStartCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// This might fail because of SSH validation, which is expected in a clean environment
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Validating GitHub SSH connection")
}
