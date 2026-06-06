//go:build !darwin

package platform

import "errors"

type unsupportedSetup struct{}

func current() Setup {
	return unsupportedSetup{}
}

func (unsupportedSetup) EnsureInterfaceAlias(req SetupRequest, log func(string)) error {
	return errors.New("automatic interface setup is currently implemented only on macOS")
}

func (unsupportedSetup) EnsureDHCPHelper(req SetupRequest, executable string, log func(string)) error {
	return errors.New("automatic DHCP helper setup is currently implemented only on macOS")
}

func (unsupportedSetup) EnsureTFTPImage(req SetupRequest, log func(string)) error {
	return errors.New("automatic TFTP setup is currently implemented only on macOS")
}

func (unsupportedSetup) EnsureTFTPService(log func(string)) error {
	return errors.New("automatic TFTP setup is currently implemented only on macOS")
}

func (unsupportedSetup) SerialInUse(port string) (bool, string) {
	return false, ""
}
