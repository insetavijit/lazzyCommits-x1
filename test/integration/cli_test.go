package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ResponseEnvelope struct {
	Version   string      `json:"version"`
	Timestamp string      `json:"timestamp"`
	Command   string      `json:"command"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

func TestBriefCommand(t *testing.T) {
	// Build the binary first
	cmdBuild := exec.Command("go", "build", "-o", "../../lazycommit_test", "../../main.go")
	err := cmdBuild.Run()
	assert.NoError(t, err)

	// Run the brief command
	cmd := exec.Command("../../lazycommit_test", "brief", ".")
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	assert.NoError(t, err)
    
    // Cleanup
    defer os.Remove("../../lazycommit_test")

	var envelope ResponseEnvelope
	err = json.Unmarshal(out.Bytes(), &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "brief", envelope.Command)
}
