package tftphelper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

// Verify downloads the exact image through TFTP before any AP inventory or flash
// changes. A listening UDP port alone does not prove that the intended file is served.
func Verify(address, name, expectedSHA string, timeout time.Duration) error {
	remote, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	request := append([]byte{0, opRRQ}, []byte(name+"\x00octet\x00")...)
	outgoing := request
	destination := remote
	var peer *net.UDPAddr
	expectedBlock := uint16(1)
	hash := sha256.New()
	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 1500)
	for time.Now().Before(deadline) {
		if _, err := conn.WriteToUDP(outgoing, destination); err != nil {
			return err
		}
		readDeadline := time.Now().Add(500 * time.Millisecond)
		if readDeadline.After(deadline) {
			readDeadline = deadline
		}
		_ = conn.SetReadDeadline(readDeadline)
		n, from, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		if !from.IP.Equal(remote.IP) || (peer != nil && from.Port != peer.Port) || n < 4 {
			continue
		}
		opcode := binary.BigEndian.Uint16(buffer[:2])
		if opcode == opERROR {
			return fmt.Errorf("TFTP server rejected %s: %s", name, buffer[4:n])
		}
		if opcode != opDATA || n > 516 {
			return fmt.Errorf("unexpected TFTP response for %s", name)
		}
		block := binary.BigEndian.Uint16(buffer[2:4])
		if block != expectedBlock {
			continue
		}
		peer, destination = from, from
		_, _ = hash.Write(buffer[4:n])
		outgoing = []byte{0, opACK, buffer[2], buffer[3]}
		expectedBlock++
		if n < 516 {
			if _, err := conn.WriteToUDP(outgoing, destination); err != nil {
				return err
			}
			if hex.EncodeToString(hash.Sum(nil)) != expectedSHA {
				return fmt.Errorf("TFTP-served firmware does not match the approved SHA256")
			}
			return nil
		}
	}
	return fmt.Errorf("TFTP verification timed out for %s at %s", name, address)
}
