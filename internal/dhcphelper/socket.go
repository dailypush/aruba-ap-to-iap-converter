package dhcphelper

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

func listenOnInterface(iface *net.Interface) (*net.UDPConn, error) {
	lc := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var optionErr error
		if err := raw.Control(func(fd uintptr) { optionErr = bindInterface(fd, iface) }); err != nil {
			return err
		}
		return optionErr
	}}
	conn, err := lc.ListenPacket(context.Background(), "udp4", ":67")
	if err != nil {
		return nil, fmt.Errorf("bind DHCP to %s: %w", iface.Name, err)
	}
	return conn.(*net.UDPConn), nil
}
