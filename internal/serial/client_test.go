package serial

import (
	"io"
	"regexp"
	"testing"
	"time"
)

type chunkPort struct {
	chunks  [][]byte
	failure error
}

func (p *chunkPort) Read(b []byte) (int, error) {
	if len(p.chunks) == 0 {
		if p.failure != nil {
			return 0, p.failure
		}
		time.Sleep(time.Millisecond)
		return 0, io.EOF
	}
	n := copy(b, p.chunks[0])
	p.chunks[0] = p.chunks[0][n:]
	if len(p.chunks[0]) == 0 {
		p.chunks = p.chunks[1:]
	}
	return n, nil
}
func (*chunkPort) Write(b []byte) (int, error) { return len(b), nil }
func (*chunkPort) Close() error                { return nil }
func TestReadUntilFragmentedPrompt(t *testing.T) {
	port := &chunkPort{chunks: [][]byte{[]byte("Card 0: Sys"), []byte("tem\nSerial: ABC\napbo"), []byte("ot> ")}}
	client := &Client{port: port}
	out, err := client.ReadUntil(regexp.MustCompile(`apboot> $`), time.Second)
	if err != nil || out != "Card 0: System\nSerial: ABC\napboot> " {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
func TestReadUntilReportsDisconnect(t *testing.T) {
	client := &Client{port: &chunkPort{chunks: [][]byte{[]byte("partial output")}, failure: io.ErrClosedPipe}}
	out, err := client.ReadUntil(regexp.MustCompile(`apboot>`), time.Second)
	if err != io.ErrClosedPipe || out != "partial output" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
