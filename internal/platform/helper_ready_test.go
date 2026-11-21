package platform

import (
	"os"
	"path/filepath"
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
