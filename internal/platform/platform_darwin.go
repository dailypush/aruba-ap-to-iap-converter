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

func (darwinSetup) EnsureDHCPHelper(req SetupRequest, executable string, log func(string)) error {
	if executable == "" {
		return fmt.Errorf("cannot start DHCP helper: executable path is empty")
	}
	logPath := req.LogPath
	if logPath == "" {
		logPath = filepath.Join("/tmp", "iap325-dhcp-helper.log")
	}
	cmd := fmt.Sprintf(
		"nohup %s dhcp-helper --iface %s --server-ip %s --lease-ip %s --netmask %s --duration 900 >> %s 2>&1 &",
		shell(executable),
		shell(req.Interface),
		shell(req.ServerIP),
		shell(req.LeaseIP),
		shell(req.Netmask),
		shell(logPath),
	)
	log("Starting direct-connect DHCP helper with admin approval")
	return osascriptAdmin(cmd)
}

func (darwinSetup) EnsureTFTPImage(req SetupRequest, log func(string)) error {
	cmd := fmt.Sprintf("mkdir -p %s && cp %s %s && chmod 644 %s", shellDir(req.TFTPPath), shell(req.FirmwarePath), shell(req.TFTPPath), shell(req.TFTPPath))
	log("Ensuring TFTP image with admin approval: " + req.TFTPPath)
	return osascriptAdmin(cmd)
}

func (darwinSetup) EnsureTFTPService(log func(string)) error {
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
	escaped := strings.ReplaceAll(command, `"`, `\"`)
	return exec.Command("osascript", "-e", `do shell script "`+escaped+`" with administrator privileges`).Run()
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
