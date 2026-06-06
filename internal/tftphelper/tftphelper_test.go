package tftphelper

import (
	"path/filepath"
	"testing"
)

func TestSafePathAllowsNestedArubaImage(t *testing.T) {
	root := t.TempDir()
	got, err := safePath(root, "aruba/ArubaInstant_Hercules_6.5.4.3_61959")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "aruba", "ArubaInstant_Hercules_6.5.4.3_61959")
	if got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
}

func TestSafePathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := safePath(root, "../secret"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestParseRRQ(t *testing.T) {
	pkt := []byte{0, opRRQ}
	pkt = append(pkt, []byte("aruba/image\x00octet\x00")...)
	name, mode, err := parseRRQ(pkt)
	if err != nil {
		t.Fatal(err)
	}
	if name != "aruba/image" || mode != "octet" {
		t.Fatalf("name=%q mode=%q", name, mode)
	}
}
