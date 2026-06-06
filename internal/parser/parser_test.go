package parser

import "testing"

func TestParseMfgInfo(t *testing.T) {
	text := `Inventory:
Card 0: System
	Serial              : DD0056915
	Wired MAC           : 40:e3:d6:ce:16:38
Card 1: CPU
	Serial              : BF4600E55`
	got, err := ParseMfgInfo(text)
	if err != nil {
		t.Fatal(err)
	}
	if got.SystemSerial != "DD0056915" {
		t.Fatalf("serial=%q", got.SystemSerial)
	}
	if got.WiredMAC != "40:e3:d6:ce:16:38" {
		t.Fatalf("mac=%q", got.WiredMAC)
	}
}

func TestParseMfgInfoWithPromptNoise(t *testing.T) {
	text := `apboot> mfginfo
Inventory:
Card 0: System
	Date Code           : 111315
	Serial              : DD0053500
	Wired MAC           : 40:e3:d6:cd:fb:8a
Card 1: CPU`
	got, err := ParseMfgInfo(text)
	if err != nil {
		t.Fatal(err)
	}
	if got.SystemSerial != "DD0053500" {
		t.Fatalf("serial=%q", got.SystemSerial)
	}
}

func TestCountryCodeCommand(t *testing.T) {
	got := CountryCodeCommand("US", "DD0057350")
	want := "proginv system ccode CCODE-US-6e0a0d857dea9955747bcd6cf4659afb3a708074"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDecodeDefaultIP(t *testing.T) {
	got, err := DecodeDefaultIP("a9fe8d06")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "169.254.141.6" {
		t.Fatalf("got %s", got)
	}
}

func TestParseDefaultIPWithSerialChunkSplits(t *testing.T) {
	text := "Jan 1\n[ 134.5] Picked up\n default IP a9f\ne8d06\n"
	got, ok := ParseDefaultIP(text)
	if !ok {
		t.Fatal("expected default IP")
	}
	if got.String() != "169.254.141.6" {
		t.Fatalf("got %s", got)
	}
}

func TestCountOSInfoVersion(t *testing.T) {
	text := "Partition 0:\n version: 6.5.4.3-6.5.4.3\nPartition 1:\n version: 6.5.4.3-6.5.4.3\n"
	if CountOSInfoVersion(text, "6.5.4.3") != 2 {
		t.Fatal("expected two version matches")
	}
}
