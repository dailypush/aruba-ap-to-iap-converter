//go:build darwin

package platform

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type darwinSetup struct{}

func current() Setup {
	return darwinSetup{}
}

func (darwinSetup) TFTPPaths(rootDir, imageName string) (string, string) {
	return filepath.Join("/private/tftpboot", "aruba", imageName), "aruba/" + imageName
}

func (darwinSetup) EnsureInterfaceAlias(req SetupRequest, log func(string)) error {
	out, err := exec.Command("ifconfig", req.Interface).CombinedOutput()
	if err != nil {
		return err
	}
	if strings.Contains(string(out), "inet "+req.ServerIP+" ") {
		log("[OK] " + req.Interface + " has " + req.ServerIP)
		return nil
	}
	cmd := fmt.Sprintf("ifconfig %s alias %s netmask %s", shell(req.Interface), shell(req.ServerIP), shell(req.Netmask))
	log("Adding direct-connect alias with admin approval: " + cmd)
	return osascriptAdmin(cmd)
}

func (darwinSetup) EnsureDHCPHelper(req SetupRequest, executable string, log func(string)) (func() error, error) {
	if executable == "" {
		return nil, fmt.Errorf("cannot start DHCP helper: executable path is empty")
	}
	logPath := req.LogPath
	if logPath == "" {
		logPath = filepath.Join("/tmp", "iap325-dhcp-helper.log")
	}
	return waitForHelper(logPath, func(readyPath string) error {
		cmd := backgroundCommand(executable, []string{
			"dhcp-helper", "--iface", req.Interface,
			"--server-ip", req.ServerIP, "--lease-ip", req.LeaseIP,
			"--netmask", req.Netmask, "--duration", "3600", "--ready-file", readyPath,
		}, logPath)
		log("Starting direct-connect DHCP helper with admin approval")
		return osascriptAdmin(cmd)
	})
}

func (darwinSetup) EnsureTFTPImage(req SetupRequest, log func(string)) error {
	cmd := fmt.Sprintf("mkdir -p %s && cp %s %s && chmod 644 %s", shellDir(req.TFTPPath), shell(req.FirmwarePath), shell(req.TFTPPath), shell(req.TFTPPath))
	log("Ensuring TFTP image with admin approval: " + req.TFTPPath)
	return osascriptAdmin(cmd)
}

func (darwinSetup) EnsureTFTPService(req SetupRequest, log func(string)) error {
	if exec.Command("lsof", "-nP", "-iUDP:69").Run() == nil {
		log("[OK] UDP/69 appears active")
		return nil
	}
	cmd := "launchctl bootstrap system /System/Library/LaunchDaemons/tftp.plist 2>/dev/null || true; launchctl enable system/com.apple.tftpd; launchctl kickstart -k system/com.apple.tftpd"
	log("Starting macOS tftpd with admin approval")
	return osascriptAdmin(cmd)
}

func (darwinSetup) SerialInUse(port string) (bool, string) {
	out, err := exec.Command("lsof", port).CombinedOutput()
	if err != nil || len(out) == 0 {
		return false, ""
	}
	return true, string(out)
}

func osascriptAdmin(command string) error {
	escaped := strings.ReplaceAll(command, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	out, err := exec.Command("osascript", "-e", `do shell script "`+escaped+`" with administrator privileges`).CombinedOutput()
	if err != nil {
		return fmt.Errorf("administrator command failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func shell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shellDir(path string) string {
	var b bytes.Buffer
	b.WriteString("$(dirname ")
	b.WriteString(shell(path))
	b.WriteString(")")
	return b.String()
}

// AppleScript supplies no controlling terminal. macOS nohup can fail while
// detaching from it, before the helper executes. Ignore HUP in the child shell
// and explicitly detach all three standard streams instead.
func backgroundCommand(executable string, args []string, logPath string) string {
	words := []string{shell(executable)}
	for _, arg := range args {
		words = append(words, shell(arg))
	}
	child := "trap '' HUP; exec " + strings.Join(words, " ")
	return "/bin/sh -c " + shell(child) + " </dev/null >> " + shell(logPath) + " 2>&1 &"
}
