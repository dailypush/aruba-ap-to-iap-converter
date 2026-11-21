package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelperStartupAndCleanup(t *testing.T) {
	var sessionPath string
	stopped := make(chan struct{})
	cleanup, err := waitForHelper(filepath.Join(t.TempDir(), "helper.log"), func(path string) error {
		sessionPath = path
		if err := os.WriteFile(path, []byte("ready"), 0600); err != nil {
			return err
		}
		go func() {
			defer close(stopped)
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(path)
				if string(data) == "stop" {
					_ = os.WriteFile(path, []byte("stopped"), 0600)
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	<-stopped
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("session file not removed: %v", err)
	}
}

func TestHelperStartupFailureIncludesNewLogOutput(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "helper.log")
	if err := os.WriteFile(logPath, []byte("OLD FAILURE\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := waitForHelper(logPath, func(path string) error {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = f.WriteString("bind DHCP: address already in use\n")
		_ = f.Close()
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte("stopped"), 0600)
	})
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("missing underlying error: %v", err)
	}
	if strings.Contains(err.Error(), "OLD FAILURE") {
		t.Fatalf("included stale failure: %v", err)
	}
}
