package app

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const maxLiveLogBytes = 128 * 1024

// Keep the full transcript on disk; bound only the live display and its queue.
func logTail(s string) string {
	if len(s) <= maxLiveLogBytes {
		return s
	}
	s = s[len(s)-maxLiveLogBytes:]
	for len(s) > 0 && !utf8.RuneStart(s[0]) {
		s = s[1:]
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	return s
}

type logBuffer struct {
	mu        sync.Mutex
	pending   string
	scheduled bool
}

func (b *logBuffer) append(s string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = logTail(b.pending + s)
	if b.scheduled {
		return false
	}
	b.scheduled = true
	return true
}

func (b *logBuffer) drain() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := b.pending
	b.pending = ""
	b.scheduled = false
	return s
}

// Serial reads are arbitrary chunks. Normalize line endings only once the
// chunks have been joined, so a CRLF split across reads stays one newline.
func displayLog(s string) string {
	s = serialLineEnding.ReplaceAllString(s, "\n")
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, s)
}

var serialLineEnding = regexp.MustCompile(`\r+\n?`)
