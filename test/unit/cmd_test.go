package cmd_test

import (
	"os"
	"testing"

	"github.com/lazycommit/lazycommit/cmd"
	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-root-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	// Test Execute without args (will show help)
	os.Args = []string{"lazycommit", "--help"}
	err := cmd.Execute()
	assert.NoError(t, err)
}

func TestExecute_WithConfig(t *testing.T) {
	// Mock HOME
	tmpDir, _ := os.MkdirTemp("", "lazycommit-config-test-*")
	defer os.RemoveAll(tmpDir)
	
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	configPath := tmpDir + "/config.toml"
	os.WriteFile(configPath, []byte(`[daemon]
debug = true`), 0644)

	os.Args = []string{"lazycommit", "--config", configPath, "--help"}
	err := cmd.Execute()
	assert.NoError(t, err)
}
