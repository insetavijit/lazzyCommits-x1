package core

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type ResponseEnvelope struct {
	Version   string      `json:"version"`
	Timestamp time.Time   `json:"timestamp"`
	Command   string      `json:"command"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

func PrintJSON(command string, data interface{}) {
	envelope := ResponseEnvelope{
		Version:   "1.1.0",
		Timestamp: time.Now(),
		Command:   command,
		Data:      data,
	}
	printEnvelope(envelope)
}

func PrintErrorJSON(command string, err error) {
	envelope := ResponseEnvelope{
		Version:   "1.1.0",
		Timestamp: time.Now(),
		Command:   command,
		Error:     err.Error(),
	}
	printEnvelope(envelope)
}

func printEnvelope(v interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}
