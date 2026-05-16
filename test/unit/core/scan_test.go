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

func TestNewScanCmd(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "lazycommit-scan-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cmd := core.NewScanCmd()
	
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
	assert.Equal(t, "scan", envelope.Command)
	
	// Data should be a list or nil (likely nil since we didn't init git)
	if envelope.Data != nil {
		_, ok := envelope.Data.([]interface{})
		assert.True(t, ok)
	}
}

func TestNewScanCmd_Empty(t *testing.T) {
	cmd := core.NewScanCmd()
	
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Use an empty path
	cmd.SetArgs([]string{""})
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
	assert.Equal(t, "scan", envelope.Command)
}
