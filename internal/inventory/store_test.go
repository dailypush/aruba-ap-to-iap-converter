package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInventoryUpsertReplacesSerial(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	base := Result{
		Timestamp:    time.Date(2026, 6, 5, 1, 0, 0, 0, time.FixedZone("EDT", -4*3600)),
		Serial:       "DD1",
		WiredMAC:     "aa",
		Country:      "US",
		ImageVersion: "6.4.4.8",
		Status:       "SUCCESS",
		LogPath:      "old.log",
	}
	if err := s.AppendResult(base); err != nil {
		t.Fatal(err)
	}
	base.ImageVersion = "6.5.4.3"
	base.LogPath = "new.log"
	if err := s.AppendResult(base); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "ap-inventory.csv"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if strings.Count(text, "DD1") != 1 {
		t.Fatalf("expected one inventory row for DD1:\n%s", text)
	}
	if !strings.Contains(text, "6.5.4.3") {
		t.Fatalf("missing updated version:\n%s", text)
	}
}
