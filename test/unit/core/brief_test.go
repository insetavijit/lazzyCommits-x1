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

func TestNewBriefCmd(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "lazycommit-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	absPath, _ := filepath.Abs(tmpDir)

	cmd := core.NewBriefCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{tmpDir})
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
	assert.Equal(t, "brief", envelope.Command)

	// Verify Data contains the repo path
	dataMap := envelope.Data.(map[string]interface{})
	assert.Equal(t, absPath, dataMap["path"])
}

func TestNewBriefCmd_DefaultPath(t *testing.T) {
	cmd := core.NewBriefCmd()
	
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
	assert.Equal(t, "brief", envelope.Command)
}
