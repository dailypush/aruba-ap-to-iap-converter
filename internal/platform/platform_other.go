//go:build !darwin && !windows

package platform

import (
	"errors"
	"path/filepath"
)

type unsupportedSetup struct{}

func current() Setup {
	return unsupportedSetup{}
}

func (unsupportedSetup) TFTPPaths(rootDir, imageName string) (string, string) {
	return filepath.Join(rootDir, "tftpboot", "aruba", imageName), "aruba/" + imageName
}

func (unsupportedSetup) EnsureInterfaceAlias(req SetupRequest, log func(string)) error {
	return errors.New("automatic interface setup is currently implemented only on macOS and Windows")
}

func (unsupportedSetup) EnsureDHCPHelper(req SetupRequest, executable string, log func(string)) error {
	return errors.New("automatic DHCP helper setup is currently implemented only on macOS and Windows")
}

func (unsupportedSetup) EnsureTFTPImage(req SetupRequest, log func(string)) error {
	return errors.New("automatic TFTP setup is currently implemented only on macOS and Windows")
}

func (unsupportedSetup) EnsureTFTPService(req SetupRequest, log func(string)) error {
	return errors.New("automatic TFTP setup is currently implemented only on macOS and Windows")
}

func (unsupportedSetup) SerialInUse(port string) (bool, string) {
	return false, ""
}
