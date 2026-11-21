//go:build windows

package dhcphelper

import (
	"encoding/binary"
	"golang.org/x/sys/windows"
	"net"
)

func bindInterface(fd uintptr, iface *net.Interface) error {
	// Winsock IP_IFLIST limits incoming datagrams; IP_UNICAST_IF selects egress.
	// Windows SDK ws2ipdef.h; requires Windows 10 1803 or newer.
	// https://learn.microsoft.com/en-us/windows/win32/winsock/ipproto-ip-socket-options
	const ipIfList, ipAddIfList, ipUnicastIf = 28, 29, 31
	sock := windows.Handle(fd)
	if err := windows.SetsockoptInt(sock, windows.IPPROTO_IP, ipIfList, 1); err != nil {
		return err
	}
	if err := windows.SetsockoptInt(sock, windows.IPPROTO_IP, ipAddIfList, iface.Index); err != nil {
		return err
	}
	// Unlike IP_ADD_IFLIST, IP_UNICAST_IF takes its index in network byte order.
	var index [4]byte
	binary.BigEndian.PutUint32(index[:], uint32(iface.Index))
	return windows.SetsockoptInt(sock, windows.IPPROTO_IP, ipUnicastIf, int(binary.NativeEndian.Uint32(index[:])))
}
