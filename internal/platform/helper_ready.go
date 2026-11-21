package platform

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The session file acknowledges startup and requests shutdown without killing
// arbitrary processes or prompting for administrator rights a second time.
func waitForHelper(logPath string, start func(readyPath string) error) (func() error, error) {
	var logOffset int64
	if info, err := os.Stat(logPath); err == nil {
		logOffset = info.Size()
	}
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
	return nil, helperStartupError(logPath, logOffset)
}

func helperStartupError(logPath string, offset int64) error {
	message := "DHCP helper did not become ready"
	if f, err := os.Open(logPath); err == nil {
		defer f.Close()
		if st, err := f.Stat(); err == nil {
			if st.Size() < offset {
				offset = 0
			}
			if st.Size()-offset > 4096 {
				offset = st.Size() - 4096
			}
			if _, err := f.Seek(offset, io.SeekStart); err == nil {
				if data, err := io.ReadAll(io.LimitReader(f, 4096)); err == nil && strings.TrimSpace(string(data)) != "" {
					message += ":\n" + strings.TrimSpace(string(data))
				}
			}
		}
	}
	return fmt.Errorf("%s\nHelper log: %s", message, logPath)
}
