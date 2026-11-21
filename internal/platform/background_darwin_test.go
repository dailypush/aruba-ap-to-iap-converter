//go:build darwin

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBackgroundHelperWithoutTerminal(t *testing.T) {
	for _, mode := range []string{"shell", "applescript"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "applescript" && os.Getenv("IAP325_APPLESCRIPT_SMOKE") == "" {
				t.Skip("opt-in native AppleScript launcher check")
			}
			path := filepath.Join(t.TempDir(), "helper's log.txt")
			command := backgroundCommand("/bin/sh", []string{"-c", `printf '%s' "$1"; printf '%s' "$2" >&2`, "helper", "ready' $literal", " error-output"}, path)
			cmd := exec.Command("/bin/sh", "-c", command)
			if mode == "applescript" {
				cmd = exec.Command("osascript", "-e", "do shell script "+strconv.Quote(command))
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("launch: %v: %s", err, output)
			}
			if strings.TrimSpace(string(output)) != "" {
				t.Fatalf("helper leaked output to launcher: %q", output)
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				data, err := os.ReadFile(path)
				if err == nil && string(data) == "ready' $literal error-output" {
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			data, _ := os.ReadFile(path)
			t.Fatalf("helper did not produce expected output: %q", data)
		})
	}
}
