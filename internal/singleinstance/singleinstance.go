package singleinstance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// LockFile attempts to acquire a single-instance lock.
// Returns an error if another instance is already running.
func LockFile() (unlock func(), err error) {
	lockPath, err := lockFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get lock file path: %w", err)
	}

	// Check if another instance is already running
	if data, err := os.ReadFile(lockPath); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if isProcessAlive(pid) {
				return nil, fmt.Errorf("RelayAI is already running (PID: %d)", pid)
			}
		}
	}

	// Write current PID
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		return nil, fmt.Errorf("failed to write lock file: %w", err)
	}

	unlock = func() {
		os.Remove(lockPath)
	}
	return unlock, nil
}

func lockFilePath() (string, error) {
	var dir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "RelayAI")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dir = filepath.Join(appData, "RelayAI")
	default: // linux
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".relayai")
	}
	return filepath.Join(dir, "relayai.lock"), nil
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
