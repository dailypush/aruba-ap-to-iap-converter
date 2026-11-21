package app

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestLiveLogIsBounded(t *testing.T) {
	var b logBuffer
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				b.append(strings.Repeat("é", 1024) + "\n")
			}
		}()
	}
	wg.Wait()
	out := b.drain()
	if len(out) > maxLiveLogBytes || !utf8.ValidString(out) {
		t.Fatalf("invalid live log, %d bytes", len(out))
	}
	if !b.append("next\n") {
		t.Fatal("new flush was not scheduled")
	}
	if got := b.drain(); got != "next\n" {
		t.Fatalf("got %q", got)
	}
}

func TestLiveLogPreservesSerialLinesAcrossReads(t *testing.T) {
	var b logBuffer
	b.append("ARP Retry co")
	b.append("unt exceeded\r\r")
	firstBatch := b.drain()
	b.append("\neth0: link up\r")
	b.append("\n\x13Using eth0 device\r\n")
	got := displayLog(firstBatch + b.drain())
	want := "ARP Retry count exceeded\neth0: link up\nUsing eth0 device\n"
	if got != want {
		t.Fatalf("display output = %q, want %q", got, want)
	}
}
