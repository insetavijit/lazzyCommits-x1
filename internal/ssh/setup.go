package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SetupResult struct {
	Success    bool   `json:"success"`
	KeyPath    string `json:"keyPath"`
	PublicKey  string `json:"publicKey"`
	Message    string `json:"message"`
	Configured bool   `json:"configured"`
}

type VerifyResult struct {
	Success bool   `json:"success"`
	User    string `json:"user"`
	Message string `json:"message"`
}

func Setup(email string) (*SetupResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	keyPath := filepath.Join(sshDir, "id_ed25519")
	pubPath := keyPath + ".pub"
	res := &SetupResult{KeyPath: keyPath}

	// 1. Generate Key if missing
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", email, "-f", keyPath, "-N", "")
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("failed to generate SSH key: %v (output: %s)", err, string(out))
		}
		res.Message = "New SSH key generated."
	} else {
		res.Message = "Existing SSH key found."
	}

	// Read public key
	pubKey, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}
	res.PublicKey = strings.TrimSpace(string(pubKey))

	// 2. Add GitHub to known_hosts
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	if !hostInKnownHosts(knownHostsPath, "github.com") {
		cmd := exec.Command("ssh-keyscan", "-H", "github.com")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err == nil {
				f.Write(out)
				f.Close()
				res.Message += " GitHub added to known_hosts."
			}
		}
	}

	// 3. Configure ~/.ssh/config
	configPath := filepath.Join(sshDir, "config")
	if err := configureSSH(configPath, keyPath); err != nil {
		res.Message += fmt.Sprintf(" Warning: config update failed: %v", err)
	} else {
		res.Configured = true
		res.Message += " SSH config updated for github.com."
	}

	res.Success = true
	return res, nil
}

func VerifyConnection() (*VerifyResult, error) {
	cmd := exec.Command("ssh", "-T", "git@github.com", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5")
	var out bytes.Buffer
	cmd.Stderr = &out
	cmd.Stdout = &out

	_ = cmd.Run()
	output := out.String()

	if strings.Contains(output, "successfully authenticated") {
		user := "unknown"
		parts := strings.Split(output, " ")
		if len(parts) >= 2 {
			// Hi username! You've successfully authenticated...
			user = strings.Trim(parts[1], "!,")
		}
		return &VerifyResult{
			Success: true,
			User:    user,
			Message: "Successfully authenticated with GitHub.",
		}, nil
	}

	return &VerifyResult{
		Success: false,
		Message: output,
	}, nil
}

func hostInKnownHosts(path, host string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(host))
}

func configureSSH(path, keyPath string) error {
	data, err := os.ReadFile(path)
	if err == nil && bytes.Contains(data, []byte("Host github.com")) {
		return nil // Already configured
	}

	block := fmt.Sprintf("\nHost github.com\n    HostName github.com\n    User git\n    IdentityFile %s\n    IdentitiesOnly yes\n", keyPath)
	
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(block)
	return err
}
