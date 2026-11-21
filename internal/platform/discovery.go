package platform

import (
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// DiscoverAdapters supplies editable suggestions; it never selects a network
// interface automatically because choosing the wrong adapter changes that network.
func DiscoverAdapters() (ports, interfaces []string, err error) {
	adapters, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}
	for _, adapter := range adapters {
		if adapter.Flags&(net.FlagLoopback|net.FlagPointToPoint) == 0 {
			interfaces = append(interfaces, adapter.Name)
		}
	}
	if runtime.GOOS == "windows" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, queryErr := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", "[System.IO.Ports.SerialPort]::GetPortNames()").Output()
		if queryErr != nil {
			return nil, interfaces, queryErr
		}
		ports = strings.Fields(string(out))
	} else {
		patterns := []string{"/dev/ttyUSB*", "/dev/ttyACM*"}
		if runtime.GOOS == "darwin" {
			patterns = []string{"/dev/cu.*"}
		}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			for _, match := range matches {
				if !strings.Contains(strings.ToLower(match), "bluetooth") {
					ports = append(ports, match)
				}
			}
		}
	}
	sort.Strings(ports)
	sort.Strings(interfaces)
	return ports, interfaces, nil
}
