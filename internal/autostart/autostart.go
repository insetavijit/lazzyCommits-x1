package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Install configures the daemon to start on login.
func Install() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	switch runtime.GOOS {
	case "linux":
		return installLinux(executable)
	case "darwin":
		return installMacOS(executable)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func installLinux(executable string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	serviceDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	serviceFile := filepath.Join(serviceDir, "lazycommit.service")
	content := fmt.Sprintf(`[Unit]
Description=lazyCommit Daemon
After=network.target

[Service]
ExecStart="%s" daemon
Restart=always

[Install]
WantedBy=default.target
`, executable)

	if err := os.WriteFile(serviceFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write systemd service file: %w", err)
	}

	// Enable and start the service
	cmd := exec.Command("systemctl", "--user", "enable", "--now", "lazycommit.service")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable systemd service: %s: %w", string(output), err)
	}

	return nil
}

func installMacOS(executable string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	agentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistFile := filepath.Join(agentsDir, "com.lazycommit.daemon.plist")
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.lazycommit.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
`, executable)

	if err := os.WriteFile(plistFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write launchd plist file: %w", err)
	}

	// Load the agent
	cmd := exec.Command("launchctl", "load", "-w", plistFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to load launchd agent: %s: %w", string(output), err)
	}

	return nil
}
