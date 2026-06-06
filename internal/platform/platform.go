package platform

type SetupRequest struct {
	Interface    string
	ServerIP     string
	LeaseIP      string
	Netmask      string
	FirmwarePath string
	TFTPPath     string
	TFTPRoot     string
	Executable   string
	LogPath      string
}

type Setup interface {
	TFTPPaths(rootDir, imageName string) (localPath string, apbootPath string)
	EnsureInterfaceAlias(req SetupRequest, log func(string)) error
	EnsureDHCPHelper(req SetupRequest, executable string, log func(string)) error
	EnsureTFTPImage(req SetupRequest, log func(string)) error
	EnsureTFTPService(req SetupRequest, log func(string)) error
	SerialInUse(port string) (bool, string)
}

func Current() Setup {
	return current()
}
