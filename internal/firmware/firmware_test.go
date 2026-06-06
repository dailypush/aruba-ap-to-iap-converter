package firmware

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsArubaOS8(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ArubaInstant_Hercules_8.5.0.3_72498")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(path); err == nil {
		t.Fatal("expected ArubaOS 8 rejection")
	}
}

func TestValidateAcceptsArubaOS6(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ArubaInstant_Hercules_6.5.4.3_61959")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Validate(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "6.5.4.3" {
		t.Fatalf("version=%q", got.Version)
	}
}
