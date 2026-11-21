//go:build !darwin && !windows && !linux

package dhcphelper

import (
	"fmt"
	"net"
)

func bindInterface(fd uintptr, iface *net.Interface) error {
	return fmt.Errorf("interface-isolated DHCP is unsupported on this platform")
}
