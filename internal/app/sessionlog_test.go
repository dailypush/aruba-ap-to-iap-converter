package app

import (
	"os"
	"strings"
	"sync"
	"testing"
)

func TestSessionLogPersistsSetupErrorsImmediately(t *testing.T) {
	log, err := newSessionLog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	if err := log.Write("[ERROR] Station setup failed: DHCP helper could not start"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[ERROR] Station setup failed: DHCP helper could not start") {
		t.Fatalf("missing error: %q", data)
	}
}

func TestSessionLogConcurrentWrites(t *testing.T) {
	log, err := newSessionLog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if err := log.Write("event"); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log.Path())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), " event\n"); got != 200 {
		t.Fatalf("lost events: %d", got)
	}
	if err := log.Write("after close"); err == nil {
		t.Fatal("write failure was not reported")
	}
}
