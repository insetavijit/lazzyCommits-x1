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

func TestNewValidateCmd_Path(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "lazycommit-validate-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	absPath, _ := filepath.Abs(tmpDir)

	cmd := core.NewValidateCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.SetArgs([]string{"--path", tmpDir})
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
	assert.Equal(t, "validate", envelope.Command)

	// Verify Data contains the repo path info
	dataMap := envelope.Data.(map[string]interface{})
	repoMap := dataMap["repo"].(map[string]interface{})
	assert.Equal(t, absPath, repoMap["path"])
	assert.Equal(t, "ok", dataMap["status"])
}
