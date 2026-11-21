package converter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dailypush/aruba-ap-to-iap-converter/internal/firmware"
	"github.com/dailypush/aruba-ap-to-iap-converter/internal/inventory"
	serialclient "github.com/dailypush/aruba-ap-to-iap-converter/internal/serial"
)

type fakeConsole struct {
	commands         []string
	replies          map[string]string
	bootReads        int
	consumedCommands int
	initialBoot      string
	monitorTail      string
	monitorErr       error
	bootTail         string
	bootErr          error
}

func (f *fakeConsole) Close() error               { return nil }
func (f *fakeConsole) SendLine(s string) error    { f.commands = append(f.commands, s); return nil }
func (f *fakeConsole) Drain(time.Duration) string { return "" }
func (f *fakeConsole) CatchAPBoot(time.Duration) (string, error) {
	return "DRAM: 235 MB\napboot> ", nil
}
func (f *fakeConsole) ExpectAPBoot(time.Duration) (string, error) { return "apboot> ", nil }
func (f *fakeConsole) ReadUntil(pattern *regexp.Regexp, _ time.Duration) (string, error) {
	command := f.commands[len(f.commands)-1]
	if command == "boot" {
		f.bootReads++
		if f.bootReads == 1 {
			if f.initialBoot != "" {
				return f.initialBoot, nil
			}
			return "ArubaOS Version 6.5.4.3 (build 61959)\n", nil
		}
		if f.bootReads > 2 {
			if f.monitorTail != "" || f.monitorErr != nil {
				return f.monitorTail, f.monitorErr
			}
			return "", serialclient.ErrTimeout
		}
		return f.bootTail, f.bootErr
	}
	if f.consumedCommands == len(f.commands) {
		return "", serialclient.ErrTimeout
	}
	f.consumedCommands = len(f.commands)
	out, ok := f.replies[command]
	if !ok {
		if strings.HasPrefix(command, "upgrade os") {
			out = "RSA signature verified.\ndone\napboot> "
		} else {
			out = "apboot> "
		}
	}
	if !pattern.MatchString(out) {
		return out, serialclient.ErrTimeout
	}
	return out, nil
}
func fixture(t *testing.T) (Engine, Request, *fakeConsole) {
	t.Helper()
	console := &fakeConsole{bootTail: "Default IP is ready\nUser:", replies: map[string]string{
		"mfginfo":            "Card 0: System\nSerial: DD0057350\nWired MAC: 40:e3:d6:ce:19:9e\napboot> ",
		"ping 169.254.141.5": "host 169.254.141.5 is alive\napboot> ",
		"osinfo":             "Partition 0:\n version: 6.5.4.3-6.5.4.3\nPartition 1:\n version: 6.5.4.3-6.5.4.3\napboot> ",
	}}
	dir := t.TempDir()
	engine := Engine{Store: inventory.New(dir), openConsole: func(string, int, func(string)) (Console, error) { return console, nil }}
	req := Request{Port: "fake", Country: "US", ServerIP: "169.254.141.5", APBootIP: "169.254.141.6", APBootNetmask: "255.255.255.0", TFTPImage: "aruba/image", Firmware: firmware.Info{Version: "6.5.4.3"}, LogDir: dir}
	return engine, req, console
}
func TestConversionSuccess(t *testing.T) {
	engine, req, console := fixture(t)
	result, err := engine.Run(context.Background(), req)
	if err != nil || result.Status != "SUCCESS" || result.Warning != "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if console.bootReads != 3 {
		t.Fatalf("boot reads=%d", console.bootReads)
	}
	if _, err := os.Stat(filepath.Join(req.LogDir, "ap-inventory.csv")); err != nil {
		t.Fatal(err)
	}
}
func TestConversionFailureSafeguards(t *testing.T) {
	for _, tc := range []struct {
		name              string
		change            func(*Request, *fakeConsole)
		status, forbidden string
	}{
		{"wrong device", func(r *Request, f *fakeConsole) { r.ExpectedSerial = "WRONG" }, "FAILED", "proginv"},
		{"failed upgrade", func(r *Request, f *fakeConsole) { f.replies["upgrade os 0 aruba/image"] = "upgrade failed\napboot> " }, "FAILED", "factory_reset"},
		{"wrong partition", func(r *Request, f *fakeConsole) {
			f.replies["osinfo"] = "Partition 0:\nversion: 6.5.4.3\nPartition 1:\nversion: 8.5.0.3\napboot> "
		}, "FAILED", "factory_reset"},
		{"panic after ready", func(r *Request, f *fakeConsole) { f.monitorTail = "Kernel panic: late failure" }, "FAILED", ""},
		{"disconnect after ready", func(r *Request, f *fakeConsole) { f.monitorErr = errors.New("disconnected") }, "UNVERIFIED", ""},
		{"early panic", func(r *Request, f *fakeConsole) { f.initialBoot = "Kernel panic: early failure" }, "FAILED", ""},
		{"late panic", func(r *Request, f *fakeConsole) { f.bootTail = "Kernel panic: startup failed" }, "FAILED", ""},
		{"boot timeout", func(r *Request, f *fakeConsole) { f.bootTail = ""; f.bootErr = serialclient.ErrTimeout }, "UNVERIFIED", ""},
		{"serial disconnected", func(r *Request, f *fakeConsole) { f.bootTail = ""; f.bootErr = errors.New("disconnected") }, "UNVERIFIED", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, req, console := fixture(t)
			tc.change(&req, console)
			result, err := engine.Run(context.Background(), req)
			if err == nil || result.Status != tc.status {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			for _, cmd := range console.commands {
				if tc.forbidden != "" && strings.HasPrefix(cmd, tc.forbidden) {
					t.Fatalf("unsafe command %q", cmd)
				}
			}
			if _, err := os.Stat(filepath.Join(req.LogDir, "ap-inventory.csv")); !os.IsNotExist(err) {
				t.Fatal("failed or unverified AP was inventoried as success")
			}
		})
	}
}
func TestRecordingFailurePreservesConversionOutcome(t *testing.T) {
	engine, req, _ := fixture(t)
	if err := os.Mkdir(filepath.Join(req.LogDir, "conversion-results.csv"), 0755); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Run(context.Background(), req)
	if err != nil || result.Status != "SUCCESS" || !strings.Contains(result.Warning, "Recording incomplete") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestBootReadyInSameSerialRead(t *testing.T) {
	engine, req, console := fixture(t)
	console.initialBoot = "ArubaOS Version 6.5.4.3 (build 61959)\nDefault IP is ready\nUser:"
	console.bootTail = ""
	console.bootErr = serialclient.ErrTimeout
	result, err := engine.Run(context.Background(), req)
	if err != nil || result.Status != "SUCCESS" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestTranscriptPreservesSerialReadBoundaries(t *testing.T) {
	engine, req, console := fixture(t)
	var live strings.Builder
	engine.Log = func(s string) { live.WriteString(s) }
	engine.openConsole = func(_ string, _ int, log func(string)) (Console, error) {
		log("ARP Retry co")
		log("unt exceeded\r")
		log("\n")
		return console, nil
	}
	result, err := engine.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(result.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != live.String() || !strings.Contains(live.String(), "ARP Retry count exceeded\r\n") {
		t.Fatal("serial reads were not joined consistently in the transcript and live output")
	}
}
