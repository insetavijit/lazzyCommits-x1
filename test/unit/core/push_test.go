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

func TestNewPushCmd(t *testing.T) {
	// Create a temporary directory and init a git repo
	tmpDir, err := os.MkdirTemp("", "lazycommit-push-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	absPath, _ := filepath.Abs(tmpDir)
	
	// Init git repo
	exec.Command("git", "-C", absPath, "init").Run()

	cmd := core.NewPushCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{absPath})
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
	assert.Equal(t, "push", envelope.Command)
}
