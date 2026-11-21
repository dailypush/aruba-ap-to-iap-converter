//go:build linux

package dhcphelper

import (
	"golang.org/x/sys/unix"
	"net"
)

func bindInterface(fd uintptr, iface *net.Interface) error {
	return unix.BindToDevice(int(fd), iface.Name)
}
