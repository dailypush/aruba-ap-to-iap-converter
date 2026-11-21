package platform

import (
	"net"
	"runtime"
)

type SetupRequest struct {
	Interface      string
	ServerIP       string
	LeaseIP        string
	Netmask        string
	FirmwarePath   string
	TFTPPath       string
	TFTPRoot       string
	Executable     string
	LogPath        string
	ExpectedSHA256 string
}

type Setup interface {
	TFTPPaths(rootDir, imageName string) (localPath string, apbootPath string)
	EnsureInterfaceAlias(req SetupRequest, log func(string)) error
	EnsureDHCPHelper(req SetupRequest, executable string, log func(string)) (func() error, error)
	EnsureTFTPImage(req SetupRequest, log func(string)) error
	EnsureTFTPService(req SetupRequest, log func(string)) error
	SerialInUse(port string) (bool, string)
}

func Current() Setup {
	return current()
}

// TFTPVerifyAddress is the endpoint for a local service check, separate from
// the server address sent to APBoot. macOS launchd serves TFTP on loopback;
// accessing a local link-local alias can hit an Ethernet/ARP reject route.
// Windows binds the bundled helper to the selected server address instead.
func TFTPVerifyAddress(serverIP string) string {
	return tftpVerifyAddress(runtime.GOOS, serverIP)
}

func tftpVerifyAddress(goos, serverIP string) string {
	if goos == "darwin" {
		return "127.0.0.1:69"
	}
	return net.JoinHostPort(serverIP, "69")
}
