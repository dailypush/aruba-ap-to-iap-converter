//go:build darwin

package dhcphelper

import (
	"golang.org/x/sys/unix"
	"net"
)

func bindInterface(fd uintptr, iface *net.Interface) error {
	return unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, iface.Index)
}
