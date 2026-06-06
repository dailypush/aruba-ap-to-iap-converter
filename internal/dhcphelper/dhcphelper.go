package dhcphelper

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

type Config struct {
	Interface string
	ServerIP  net.IP
	LeaseIP   net.IP
	Netmask   net.IP
	Duration  time.Duration
	Log       func(string)
}

func Run(cfg Config) error {
	if cfg.ServerIP == nil || cfg.LeaseIP == nil || cfg.Netmask == nil {
		return errors.New("server IP, lease IP, and netmask are required")
	}
	if cfg.Duration == 0 {
		cfg.Duration = 15 * time.Minute
	}
	log := func(s string) {
		if cfg.Log != nil {
			cfg.Log(s)
		}
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 67})
	if err != nil {
		return fmt.Errorf("bind DHCP UDP/67: %w", err)
	}
	defer conn.Close()
	if err := conn.SetReadBuffer(8192); err != nil {
		log("warning: " + err.Error())
	}
	deadline := time.Now().Add(cfg.Duration)
	log(fmt.Sprintf("DHCP helper listening on %s; offering %s from %s for %s", valueOr(cfg.Interface, "direct interface"), cfg.LeaseIP, cfg.ServerIP, cfg.Duration))

	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 1500)
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		msg, err := parse(buf[:n])
		if err != nil || msg.messageType == 0 {
			continue
		}
		switch msg.messageType {
		case 1:
			log("DHCPDISCOVER from " + msg.hwaddr.String() + " via " + remote.String())
			if err := send(conn, msg, cfg, 2); err != nil {
				log("DHCPOFFER failed: " + err.Error())
			} else {
				log("DHCPOFFER " + cfg.LeaseIP.String() + " to " + msg.hwaddr.String())
			}
		case 3:
			log("DHCPREQUEST from " + msg.hwaddr.String() + " via " + remote.String())
			if err := send(conn, msg, cfg, 5); err != nil {
				log("DHCPACK failed: " + err.Error())
			} else {
				log("DHCPACK " + cfg.LeaseIP.String() + " to " + msg.hwaddr.String())
			}
		}
	}
	log("DHCP helper exiting after " + cfg.Duration.String())
	return nil
}

type message struct {
	xid         []byte
	flags       []byte
	hwaddr      net.HardwareAddr
	chaddr      []byte
	messageType byte
}

func parse(pkt []byte) (message, error) {
	var msg message
	if len(pkt) < 240 || pkt[0] != 1 || pkt[1] != 1 || pkt[2] == 0 {
		return msg, errors.New("not an Ethernet DHCP request")
	}
	if binary.BigEndian.Uint32(pkt[236:240]) != 0x63825363 {
		return msg, errors.New("missing DHCP magic")
	}
	hlen := int(pkt[2])
	if hlen > 16 {
		hlen = 16
	}
	msg.xid = append([]byte(nil), pkt[4:8]...)
	msg.flags = append([]byte(nil), pkt[10:12]...)
	msg.chaddr = append([]byte(nil), pkt[28:44]...)
	msg.hwaddr = net.HardwareAddr(append([]byte(nil), pkt[28:28+hlen]...))
	opts := pkt[240:]
	for i := 0; i < len(opts); {
		code := opts[i]
		i++
		if code == 0 {
			continue
		}
		if code == 255 {
			break
		}
		if i >= len(opts) {
			break
		}
		l := int(opts[i])
		i++
		if i+l > len(opts) {
			break
		}
		if code == 53 && l == 1 {
			msg.messageType = opts[i]
		}
		i += l
	}
	return msg, nil
}

func send(conn *net.UDPConn, req message, cfg Config, messageType byte) error {
	pkt := make([]byte, 240, 320)
	pkt[0] = 2
	pkt[1] = 1
	pkt[2] = byte(len(req.hwaddr))
	copy(pkt[4:8], req.xid)
	copy(pkt[10:12], req.flags)
	copy(pkt[16:20], cfg.LeaseIP.To4())
	copy(pkt[20:24], cfg.ServerIP.To4())
	copy(pkt[28:44], req.chaddr)
	binary.BigEndian.PutUint32(pkt[236:240], 0x63825363)

	opts := []byte{}
	opts = option(opts, 53, []byte{messageType})
	opts = option(opts, 54, cfg.ServerIP.To4())
	opts = option(opts, 51, u32(3600))
	opts = option(opts, 1, cfg.Netmask.To4())
	opts = option(opts, 3, cfg.ServerIP.To4())
	opts = option(opts, 6, cfg.ServerIP.To4())
	opts = append(opts, 255)
	pkt = append(pkt, opts...)

	_, err := conn.WriteToUDP(pkt, &net.UDPAddr{IP: net.IPv4bcast, Port: 68})
	return err
}

func option(out []byte, code byte, value []byte) []byte {
	out = append(out, code, byte(len(value)))
	return append(out, value...)
}

func u32(v uint32) []byte {
	out := make([]byte, 4)
	binary.BigEndian.PutUint32(out, v)
	return out
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
