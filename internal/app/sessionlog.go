package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Session logs start before station setup, so errors preceding serial capture
// are retained. Disk writes are synchronous; only the UI rendering is batched.
type sessionLog struct {
	mu          sync.Mutex
	file        *os.File
	partialLine bool
}

func newSessionLog(dir string) (*sessionLog, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(dir, "app-session-"+time.Now().Format("20060102-150405")+"-*.log")
	if err != nil {
		return nil, err
	}
	return &sessionLog{file: file}, nil
}

func (l *sessionLog) Path() string { return filepath.Clean(l.file.Name()) }

func (l *sessionLog) Write(message string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	prefix := ""
	if l.partialLine {
		prefix = "\n"
	}
	_, err := fmt.Fprintf(l.file, "%s%s %s\n", prefix, time.Now().Format(time.RFC3339Nano), strings.TrimRight(message, "\n"))
	if err == nil {
		l.partialLine = false
	}
	return err
}

func (l *sessionLog) WriteRaw(text string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := l.file.WriteString(text)
	if err == nil && text != "" {
		l.partialLine = !strings.HasSuffix(text, "\n")
	}
	return err
}

func (l *sessionLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}
