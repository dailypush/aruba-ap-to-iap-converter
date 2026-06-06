//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsSetup struct{}

func current() Setup {
	return windowsSetup{}
}

func (windowsSetup) TFTPPaths(rootDir, imageName string) (string, string) {
	return filepath.Join(rootDir, "tftpboot", "aruba", imageName), "aruba/" + imageName
}

func (windowsSetup) EnsureInterfaceAlias(req SetupRequest, log func(string)) error {
	if req.Interface == "" {
		return fmt.Errorf("Windows interface name is required, for example Ethernet")
	}
	out, err := exec.Command("netsh", "interface", "ipv4", "show", "addresses", req.Interface).CombinedOutput()
	if err == nil && strings.Contains(string(out), req.ServerIP) {
		log("[OK] " + req.Interface + " has " + req.ServerIP)
		return nil
	}
	log("Adding Windows direct-connect address with netsh: " + req.ServerIP + " on " + req.Interface)
	cmd := exec.Command("netsh", "interface", "ipv4", "add", "address", req.Interface, req.ServerIP, req.Netmask)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("netsh add address failed; run as Administrator or add %s/%s to %s manually: %w\n%s", req.ServerIP, req.Netmask, req.Interface, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (windowsSetup) EnsureDHCPHelper(req SetupRequest, executable string, log func(string)) error {
	if executable == "" {
		return fmt.Errorf("cannot start DHCP helper: executable path is empty")
	}
	logPath := req.LogPath
	if logPath == "" {
		logPath = filepath.Join(os.TempDir(), "iap325-dhcp-helper.log")
	}
	log("Starting direct-connect DHCP helper. Windows may require Administrator privileges for UDP/67.")
	cmd := exec.Command(executable, "dhcp-helper",
		"--iface", req.Interface,
		"--server-ip", req.ServerIP,
		"--lease-ip", req.LeaseIP,
		"--netmask", req.Netmask,
		"--duration", "900",
	)
	return startDetached(cmd, logPath)
}

func (windowsSetup) EnsureTFTPImage(req SetupRequest, log func(string)) error {
	if err := os.MkdirAll(filepath.Dir(req.TFTPPath), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(req.FirmwarePath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(req.TFTPPath, data, 0644); err != nil {
		return err
	}
	log("[OK] staged TFTP image: " + req.TFTPPath)
	return nil
}

func (windowsSetup) EnsureTFTPService(req SetupRequest, log func(string)) error {
	if req.Executable == "" {
		return fmt.Errorf("cannot start TFTP helper: executable path is empty")
	}
	if req.TFTPRoot == "" {
		return fmt.Errorf("cannot start TFTP helper: TFTP root is empty")
	}
	logPath := req.LogPath
	if logPath == "" {
		logPath = filepath.Join(os.TempDir(), "iap325-tftp-helper.log")
	}
	log("Starting bundled Windows TFTP helper on UDP/69 rooted at " + req.TFTPRoot)
	cmd := exec.Command(req.Executable, "tftp-helper",
		"--root", req.TFTPRoot,
		"--duration", "1800",
	)
	if err := startDetached(cmd, logPath); err != nil {
		return fmt.Errorf("start TFTP helper failed; run as Administrator or start a TFTP service rooted at %s: %w", req.TFTPRoot, err)
	}
	log("TFTP helper log: " + logPath)
	return nil
}

func (windowsSetup) SerialInUse(port string) (bool, string) {
	return false, ""
}
