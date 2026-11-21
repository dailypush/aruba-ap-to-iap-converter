package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// The session file acknowledges startup and requests shutdown without killing
// arbitrary processes or prompting for administrator rights a second time.
func waitForHelper(logPath string, start func(readyPath string) error) (func() error, error) {
	ready, err := os.CreateTemp(filepath.Dir(logPath), ".dhcp-session-*")
	if err != nil {
		return nil, err
	}
	path := ready.Name()
	if err := ready.Close(); err != nil {
		os.Remove(path)
		return nil, err
	}
	cleanup := func() error {
		defer os.Remove(path)
		if err := os.WriteFile(path, []byte("stop"), 0600); err != nil {
			return err
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			data, err := os.ReadFile(path)
			if err == nil && string(data) == "stopped" {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return fmt.Errorf("DHCP shutdown was not confirmed; check %s before the next AP", logPath)
	}
	if err := start(path); err != nil {
		os.Remove(path)
		return nil, err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && string(data) == "ready" {
			return cleanup, nil
		}
		if err == nil && string(data) == "stopped" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Removing the session file also asks a delayed helper to stop.
	os.Remove(path)
	return nil, fmt.Errorf("DHCP helper did not become ready; check %s (another helper may already be running)", logPath)
}
