package core_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/stretchr/testify/assert"
)

func TestPrintJSON(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	command := "test-command"
	data := map[string]string{"key": "value"}
	
	core.PrintJSON(command, data)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.Bytes()

	var envelope core.ResponseEnvelope
	err := json.Unmarshal(output, &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.0", envelope.Version)
	assert.Equal(t, command, envelope.Command)
	
	// Unmarshal Data back into map
	dataJSON, _ := json.Marshal(envelope.Data)
	var actualData map[string]string
	json.Unmarshal(dataJSON, &actualData)
	assert.Equal(t, data, actualData)
}

func TestPrintErrorJSON(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	command := "error-command"
	testErr := errors.New("something went wrong")
	
	core.PrintErrorJSON(command, testErr)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.Bytes()

	var envelope core.ResponseEnvelope
	err := json.Unmarshal(output, &envelope)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.0", envelope.Version)
	assert.Equal(t, command, envelope.Command)
	assert.Equal(t, testErr.Error(), envelope.Error)
}
