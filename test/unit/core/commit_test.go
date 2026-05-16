package core_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/stretchr/testify/assert"
)

func TestNewCommitCmd(t *testing.T) {
	// Create a temporary directory and init a git repo
	tmpDir, err := os.MkdirTemp("", "lazycommit-commit-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	absPath, _ := filepath.Abs(tmpDir)
	
	// Init git repo
	exec.Command("git", "-C", absPath, "init").Run()
	os.WriteFile(filepath.Join(absPath, "test.txt"), []byte("test"), 0644)

	cmd := core.NewCommitCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{absPath, "test message"})
	err = cmd.Execute()
	assert.NoError(t, err)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.Bytes()

	var envelope core.ResponseEnvelope
	err = json.Unmarshal(output, &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "commit", envelope.Command)
}
