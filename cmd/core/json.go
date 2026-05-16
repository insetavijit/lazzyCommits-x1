package core

import (
	"encoding/json"
	"fmt"
	"os"
)

func PrintJSON(v interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func PrintErrorJSON(err error) {
	PrintJSON(ErrorResponse{Error: err.Error()})
}
