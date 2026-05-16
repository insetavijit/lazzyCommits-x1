package core_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/stretchr/testify/assert"
)

func TestNewSSHCmd(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-ssh-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cmd := core.NewSSHCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// This will try to run ssh-keygen if no key exists
	// We might need to skip if ssh-keygen is not available
	cmd.SetArgs([]string{"test@example.com"})
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
	assert.Equal(t, "ssh", envelope.Command)
}
